// Command migrate-v3 moves the deployments made of the stacks version 3.0 removed onto dhis2-v2.
// It reads the same environment as the instance manager and ships in its image, so it runs next to
// it with the environment's own configuration. See README.md for the order of the phases. It is
// removed, together with internal/migratev3, once production has been migrated.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/internal/migratev3"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"gorm.io/gorm"
)

const usage = `usage: migrate-v3 <phase> [flags]

phases, in the order they run:
  plan      list the deployments that would be migrated and what blocks any of them; changes nothing
  migrate   with no instance manager serving: snapshot, rewrite to dhis2-v2, remove colliding releases
  deploy    with version 3.0 serving: deploy, wait until seeded, restore DATABASE_ID, remove the old database
  cleanup   after the grace period: delete the snapshots
  status    list the recorded state of every migrated deployment

flags:
`

var errUsage = errors.New("unknown phase")

func main() {
	err := run()
	if errors.Is(err, errUsage) {
		os.Exit(64)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		return errUsage
	}
	phase := os.Args[1]

	flags := flag.NewFlagSet(phase, flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flags.PrintDefaults()
	}
	concurrency := flags.Int("concurrency", 4, "deployments worked on at the same time")
	deployments := flags.String("deployments", "", "comma separated deployment ids to limit the phase to, for a canary run")
	excludeAnalyticsData := flags.Bool("exclude-analytics-data", false, "leave the rows of the analytics tables out of the dumps, as a save does")
	deployTimeout := flags.Duration("deploy-timeout", 6*time.Hour, "how long deploy waits for one deployment to be seeded and running")
	grace := flags.Duration("grace", 14*24*time.Hour, "how long a deployment has run on dhis2-v2 before cleanup deletes its snapshot")
	apiURL := flags.String("api-url", os.Getenv("HOSTNAME"), "the instance manager's API url")
	ignoreServingAPI := flags.Bool("ignore-serving-api", false, "run migrate although an instance manager answers at --api-url")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	selected, err := parseIds(*deployments)
	if err != nil {
		return err
	}
	encryptionKey, err := requireEnv("INSTANCE_PARAMETER_ENCRYPTION_KEY")
	if err != nil {
		return err
	}
	api := &migratev3.API{
		BaseURL:  strings.TrimRight(*apiURL, "/"),
		Email:    os.Getenv("ADMIN_USER_EMAIL"),
		Password: os.Getenv("ADMIN_USER_PASSWORD"),
		Client:   &http.Client{Timeout: time.Minute},
	}

	// Only migrate changes the schema: plan runs while the version before 3.0 still serves, and the
	// later phases run once version 3.0 has.
	db, err := newDB(logger, phase == "migrate")
	if err != nil {
		return err
	}

	migrator := &migratev3.Migrator{
		Logger:        logger,
		DB:            db,
		EncryptionKey: encryptionKey,
		API:           api,
		Concurrency:   *concurrency,
		Deployments:   selected,
		DeployTimeout: *deployTimeout,
	}

	switch phase {
	case "plan":
		planned, unsupported, err := migrator.Plan(ctx)
		if err != nil {
			return err
		}
		return migratev3.PrintPlan(os.Stdout, planned, unsupported)
	case "migrate":
		if !*ignoreServingAPI && api.Healthy(ctx) {
			return fmt.Errorf("an instance manager answers at %s, stop it before migrating", api.BaseURL)
		}
		snapshotter, err := newSnapshotter(ctx, logger, *excludeAnalyticsData)
		if err != nil {
			return err
		}
		migrator.Snapshotter = snapshotter
		return errors.Join(migrator.Migrate(ctx), migrator.PrintStatus(os.Stdout))
	case "deploy":
		if !api.Healthy(ctx) {
			return fmt.Errorf("no instance manager answers at %s, start version 3.0 before deploying", api.BaseURL)
		}
		return errors.Join(migrator.Deploy(ctx), migrator.PrintStatus(os.Stdout))
	case "cleanup":
		return errors.Join(migrator.Cleanup(ctx, *grace), migrator.PrintStatus(os.Stdout))
	case "status":
		return migrator.PrintStatus(os.Stdout)
	default:
		flags.Usage()
		return errUsage
	}
}

func parseIds(value string) ([]uint, error) {
	var ids []uint
	for field := range strings.SplitSeq(value, ",") {
		if field = strings.TrimSpace(field); field == "" {
			continue
		}
		id, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid deployment id %q: %v", field, err)
		}
		ids = append(ids, uint(id))
	}
	return ids, nil
}

func newDB(logger *slog.Logger, migrate bool) (*gorm.DB, error) {
	var config storage.PostgresqlConfig
	var err error
	if config.Host, err = requireEnv("DATABASE_HOST"); err != nil {
		return nil, err
	}
	port, err := requireEnv("DATABASE_PORT")
	if err != nil {
		return nil, err
	}
	if config.Port, err = strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid DATABASE_PORT %q: %v", port, err)
	}
	if config.Username, err = requireEnv("DATABASE_USERNAME"); err != nil {
		return nil, err
	}
	if config.Password, err = requireEnv("DATABASE_PASSWORD"); err != nil {
		return nil, err
	}
	if config.DatabaseName, err = requireEnv("DATABASE_NAME"); err != nil {
		return nil, err
	}
	if migrate {
		return storage.NewDatabase(logger, config)
	}
	return storage.ConnectDatabase(logger, config)
}

func newSnapshotter(ctx context.Context, logger *slog.Logger, excludeAnalyticsData bool) (migratev3.Snapshotter, error) {
	bucket, err := requireEnv("S3_BUCKET")
	if err != nil {
		return migratev3.Snapshotter{}, err
	}
	region, err := requireEnv("S3_REGION")
	if err != nil {
		return migratev3.Snapshotter{}, err
	}
	config, err := s3config.LoadDefaultConfig(ctx, s3config.WithRegion(region))
	if err != nil {
		return migratev3.Snapshotter{}, fmt.Errorf("failed to set up S3: %v", err)
	}
	client := s3.NewFromConfig(config, func(o *s3.Options) {
		o.UsePathStyle = true
		if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return migratev3.Snapshotter{
		Uploader:             storage.NewS3Client(logger, client, manager.NewUploader(client)), //nolint:staticcheck
		Bucket:               bucket,
		ExcludeAnalyticsData: excludeAnalyticsData,
	}, nil
}

func requireEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("required environment variable %q not set", key)
	}
	return value, nil
}
