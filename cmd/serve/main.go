// Package classification Instance Manager
//
// Instance Manager
//
//	Version: 0.1.0
//	License: TODO
//	Contact: <info@dhis2.org> https://github.com/dhis2-sre/im-manager
//
//	Consumes:
//	  - application/json
//
//	Produces:
//	  - application/json
//
//	SecurityDefinitions:
//	  oauth2:
//	    type: oauth2
//	    tokenUrl: /tokens
//	    refreshUrl: /refresh
//	    flow: password
//
// swagger:meta
package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/inspector"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/gin-gonic/gin"
	"github.com/go-mail/mail"
	"github.com/go-redis/redis"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"gorm.io/gorm"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/internal/log"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/event"
	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/dhis2-sre/im-manager/pkg/user"

	s3config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/integration"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("im-manager exited due to: %v", err)
		os.Exit(1)
	}
	fmt.Printf("im-manager exited without any error")
}

func run() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panicked due to: %v", r)
		}
	}()

	shutdown, err := initTracer()
	defer shutdown()
	if err != nil {
		return err
	}

	ctx := context.Background()
	logger := slog.New(log.New(slog.NewJSONHandler(os.Stdout, nil)))

	db, err := newDB(logger)
	if err != nil {
		return err
	}

	dialer, err := newDialer()
	if err != nil {
		return err
	}

	userService, err := newUserService(db, dialer)
	if err != nil {
		return err
	}

	authorization := middleware.NewAuthorization(logger, userService)

	redis, err := newRedis()
	if err != nil {
		return err
	}
	tokenRepository := token.NewRepository(redis)

	authConfig, err := newAuthenticationConfig()
	if err != nil {
		return err
	}
	privateKey, err := getPrivateKey(ctx, logger)
	if err != nil {
		return err
	}
	tokenService, err := token.NewService(logger, tokenRepository, privateKey, authConfig.AccessTokenExpirationSeconds, authConfig.RefreshTokenSecretKey, authConfig.RefreshTokenExpirationSeconds, authConfig.RefreshTokenRememberMeExpirationSeconds)
	if err != nil {
		return err
	}

	publicKey := privateKey.PublicKey
	hostname, err := requireEnv("HOSTNAME")
	if err != nil {
		return err
	}
	userHandler := user.NewHandler(logger, hostname, authConfig.SameSiteMode, authConfig.AccessTokenExpirationSeconds, authConfig.RefreshTokenExpirationSeconds, authConfig.RefreshTokenRememberMeExpirationSeconds, publicKey, userService, tokenService)

	authentication := middleware.NewAuthentication(publicKey, userService)
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)
	groupHandler := group.NewHandler(groupService)

	stackService, err := newStackService()
	if err != nil {
		return err
	}

	awsS3Client, err := newAWSS3Client(ctx)
	if err != nil {
		return err
	}

	instanceService, err := newInstanceService(logger, db, stackService, groupService, awsS3Client)
	if err != nil {
		return err
	}

	stackHandler := stack.NewHandler(stackService)

	instanceHandler, err := newInstanceHandler(stackService, groupService, instanceService)
	if err != nil {
		return err
	}

	databaseHandler, err := newDatabaseHandler(ctx, logger, db, groupService, instanceService, stackService)
	if err != nil {
		return err
	}

	err = handler.RegisterValidation()
	if err != nil {
		return err
	}

	integrationHandler, err := newIntegrationHandler()
	if err != nil {
		return err
	}

	rabbitmqConfig, err := newRabbitMQ()
	if err != nil {
		return err
	}

	eventHandler, err := newEventHandler(ctx, logger, rabbitmqConfig)
	if err != nil {
		return err
	}

	err = createGroups(ctx, logger, groupService)
	if err != nil {
		return err
	}

	err = createAdminUser(ctx, userService, groupService)
	if err != nil {
		return err
	}

	err = createE2ETestUser(ctx, userService, groupService)
	if err != nil {
		return err
	}

	ins := inspector.NewInspector(logger, instanceService, inspector.NewTTLDestroyHandler(logger, instanceService))
	// TODO: Graceful shutdown... ?
	go ins.Inspect(ctx)

	r, err := newGinEngine(logger)
	if err != nil {
		return err
	}

	r.Use(otelgin.Middleware("im")) // Attach OpenTelemetry middleware

	group.Routes(r, authentication, authorization, groupHandler)
	user.Routes(r, authentication, authorization, userHandler)
	stack.Routes(r, authentication.TokenAuthentication, stackHandler)
	integration.Routes(r, authentication, integrationHandler)
	database.Routes(r, authentication.TokenAuthentication, databaseHandler)
	instance.Routes(r, authentication.TokenAuthentication, instanceHandler)
	event.Routes(r, authentication.TokenAuthentication, eventHandler)

	logger.InfoContext(ctx, "Listening and serving HTTP")
	if err := r.Run(); err != nil {
		return fmt.Errorf("failed to start the HTTP server: %v", err)
	}
	return nil
}

func newDB(logger *slog.Logger) (*gorm.DB, error) {
	host, err := requireEnv("DATABASE_HOST")
	if err != nil {
		return nil, err
	}
	port, err := requireEnvAsInt("DATABASE_PORT")
	if err != nil {
		return nil, err
	}
	username, err := requireEnv("DATABASE_USERNAME")
	if err != nil {
		return nil, err
	}
	password, err := requireEnv("DATABASE_PASSWORD")
	if err != nil {
		return nil, err
	}
	name, err := requireEnv("DATABASE_NAME")
	if err != nil {
		return nil, err
	}

	db, err := storage.NewDatabase(
		logger,
		storage.PostgresqlConfig{Host: host, Port: port, Username: username, Password: password, DatabaseName: name},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to setup DB: %v", err)
	}

	return db, nil
}

func newDialer() (*mail.Dialer, error) {
	host, err := requireEnv("SMTP_HOST")
	if err != nil {
		return nil, err
	}
	port, err := requireEnvAsInt("SMTP_PORT")
	if err != nil {
		return nil, err
	}
	username, err := requireEnv("SMTP_USERNAME")
	if err != nil {
		return nil, err
	}
	password, err := requireEnv("SMTP_PASSWORD")
	if err != nil {
		return nil, err
	}

	return mail.NewDialer(host, port, username, password), nil
}

func newUserService(db *gorm.DB, dialer *mail.Dialer) (*user.Service, error) {
	uiURL, err := requireEnv("UI_URL")
	if err != nil {
		return nil, err
	}
	passwordTokenTTL, err := requireEnvAsUint("PASSWORD_TOKEN_TTL")
	if err != nil {
		return nil, err
	}
	userRepository := user.NewRepository(db)

	return user.NewService(uiURL, passwordTokenTTL, userRepository, dialer), nil
}

func newRedis() (*redis.Client, error) {
	host, err := requireEnv("REDIS_HOST")
	if err != nil {
		return nil, err
	}
	port, err := requireEnvAsInt("REDIS_PORT")
	if err != nil {
		return nil, err
	}

	return storage.NewRedis(host, port)
}

type authenticationConfig struct {
	SameSiteMode                            http.SameSite
	RefreshTokenSecretKey                   string
	AccessTokenExpirationSeconds            int
	RefreshTokenExpirationSeconds           int
	RefreshTokenRememberMeExpirationSeconds int
}

func newAuthenticationConfig() (authenticationConfig, error) {
	mode, err := sameSiteMode()
	if err != nil {
		return authenticationConfig{}, err
	}
	refreshTokenSecretKey, err := requireEnv("REFRESH_TOKEN_SECRET_KEY")
	if err != nil {
		return authenticationConfig{}, err
	}
	accessTokenExpirationSeconds, err := requireEnvAsInt("ACCESS_TOKEN_EXPIRATION_IN_SECONDS")
	if err != nil {
		return authenticationConfig{}, err
	}
	refreshTokenExpirationSeconds, err := requireEnvAsInt("REFRESH_TOKEN_EXPIRATION_IN_SECONDS")
	if err != nil {
		return authenticationConfig{}, err
	}
	refreshTokenRememberMeExpirationSeconds, err := requireEnvAsInt("REFRESH_TOKEN_REMEMBER_ME_EXPIRATION_IN_SECONDS")
	if err != nil {
		return authenticationConfig{}, err
	}

	return authenticationConfig{
		SameSiteMode:                            mode,
		RefreshTokenSecretKey:                   refreshTokenSecretKey,
		AccessTokenExpirationSeconds:            accessTokenExpirationSeconds,
		RefreshTokenExpirationSeconds:           refreshTokenExpirationSeconds,
		RefreshTokenRememberMeExpirationSeconds: refreshTokenRememberMeExpirationSeconds,
	}, nil
}

func sameSiteMode() (http.SameSite, error) {
	sameSiteMode, err := requireEnv("SAME_SITE_MODE")
	if err != nil {
		return 0, err
	}

	switch sameSiteMode {
	case "default":
		return http.SameSiteDefaultMode, nil
	case "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	}

	return -1, fmt.Errorf("failed to parse \"SAME_SITE_MODE\": %q", sameSiteMode)
}

func getPrivateKey(ctx context.Context, logger *slog.Logger) (*rsa.PrivateKey, error) {
	key, err := requireEnv("PRIVATE_KEY")
	if err != nil {
		return nil, err
	}
	decode, _ := pem.Decode([]byte(key))
	if decode == nil {
		return nil, errors.New("failed to decode private key")
	}

	// Openssl generates keys formatted as PKCS8 but terraform tls_private_key is producing PKCS1
	// So if parsing of PKCS8 fails we try PKCS1
	privateKey, err := x509.ParsePKCS8PrivateKey(decode.Bytes)
	if err != nil {
		if err.Error() == "x509: failed to parse private key (use ParsePKCS1PrivateKey instead for this key format)" {
			logger.InfoContext(ctx, "Trying to parse PKCS1 format...")
			privateKey, err = x509.ParsePKCS1PrivateKey(decode.Bytes)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		logger.InfoContext(ctx, "Successfully parsed private key")
	}

	return privateKey.(*rsa.PrivateKey), nil
}

func newStackService() (stack.Service, error) {
	stacks, err := stack.New(
		stack.DHIS2DB,
		stack.DHIS2Core,
		stack.DHIS2,
		stack.PgAdmin,
		stack.WhoamiGo,
		stack.IMJobRunner,
	)
	if err != nil {
		return stack.Service{}, fmt.Errorf("error in stack config: %v", err)
	}

	return stack.NewService(stacks), nil
}

func newInstanceService(logger *slog.Logger, db *gorm.DB, stackService stack.Service, groupService *group.Service, s3Client *s3.Client) (*instance.Service, error) {
	instanceParameterEncryptionKey, err := requireEnv("INSTANCE_PARAMETER_ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}
	instanceRepository := instance.NewRepository(db, instanceParameterEncryptionKey)
	classification, err := requireEnv("CLASSIFICATION")
	if err != nil {
		return nil, err
	}
	helmfileService := instance.NewHelmfileService(logger, stackService, "./stacks", classification)

	s3Bucket, err := requireEnv("S3_BUCKET")
	if err != nil {
		return nil, err
	}

	return instance.NewService(logger, instanceRepository, groupService, stackService, helmfileService, s3Client, s3Bucket), nil
}

type rabbitMQConfig struct {
	Host       string
	Port       int
	StreamPort int
	Username   string
	Password   string
}

// GetURI returns the AMQP URI for RabbitMQ.
func (r rabbitMQConfig) GetURI() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", r.Username, r.Password, r.Host, r.Port)
}

func newRabbitMQ() (rabbitMQConfig, error) {
	host, err := requireEnv("RABBITMQ_HOST")
	if err != nil {
		return rabbitMQConfig{}, err
	}
	port, err := requireEnvAsInt("RABBITMQ_PORT")
	if err != nil {
		return rabbitMQConfig{}, err
	}
	streamPort, err := requireEnvAsInt("RABBITMQ_STREAM_PORT")
	if err != nil {
		return rabbitMQConfig{}, err
	}
	username, err := requireEnv("RABBITMQ_USERNAME")
	if err != nil {
		return rabbitMQConfig{}, err
	}
	password, err := requireEnv("RABBITMQ_PASSWORD")
	if err != nil {
		return rabbitMQConfig{}, err
	}

	return rabbitMQConfig{
		Host:       host,
		Port:       port,
		StreamPort: streamPort,
		Username:   username,
		Password:   password,
	}, nil
}

func newInstanceHandler(stackService stack.Service, groupService *group.Service, instanceService *instance.Service) (instance.Handler, error) {
	defaultTTL, err := requireEnvAsUint("DEFAULT_TTL")
	if err != nil {
		return instance.Handler{}, err
	}

	return instance.NewHandler(stackService, groupService, instanceService, defaultTTL), nil
}

func newDatabaseHandler(ctx context.Context, logger *slog.Logger, db *gorm.DB, groupService *group.Service, instanceService *instance.Service, stackService stack.Service) (database.Handler, error) {
	s3Bucket, err := requireEnv("S3_BUCKET")
	if err != nil {
		return database.Handler{}, err
	}
	s3Client, err := newS3Client(ctx, logger)
	if err != nil {
		return database.Handler{}, err
	}
	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService, databaseRepository)

	return database.NewHandler(logger, databaseService, groupService, instanceService, stackService), nil
}

func newS3Client(ctx context.Context, logger *slog.Logger) (*storage.S3Client, error) {
	awsClient, err := newAWSS3Client(ctx)
	if err != nil {
		return nil, err
	}
	uploader := manager.NewUploader(awsClient)

	return storage.NewS3Client(logger, awsClient, uploader), nil
}

func newAWSS3Client(ctx context.Context) (*s3.Client, error) {
	s3Region, err := requireEnv("S3_REGION")
	if err != nil {
		return nil, err
	}

	// nolint:staticcheck
	s3Endpoint := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
			return aws.Endpoint{URL: endpoint}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})
	s3Config, err := s3config.LoadDefaultConfig(
		ctx,
		s3config.WithRegion(s3Region),
		// nolint:staticcheck
		s3config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptions(s3Endpoint)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to setup S3 config: %v", err)
	}
	s3AWSClient := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return s3AWSClient, nil
}

func newIntegrationHandler() (integration.Handler, error) {
	username, err := requireEnv("DOCKER_HUB_USERNAME")
	if err != nil {
		return integration.Handler{}, err
	}
	password, err := requireEnv("DOCKER_HUB_PASSWORD")
	if err != nil {
		return integration.Handler{}, err
	}
	dockerHubClient := integration.NewDockerHubClient(
		integration.DockerHubConfig{
			Username: username,
			Password: password,
		})

	instanceServiceHost, err := requireEnv("INSTANCE_SERVICE_HOST")
	if err != nil {
		return integration.Handler{}, err
	}

	return integration.NewHandler(dockerHubClient, instanceServiceHost), nil
}

func newEventHandler(ctx context.Context, logger *slog.Logger, rabbitmqConfig rabbitMQConfig) (event.Handler, error) {
	logger.InfoContext(ctx, "Connecting with RabbitMQ stream client", "host", rabbitmqConfig.Host, "port", rabbitmqConfig.StreamPort)
	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(rabbitmqConfig.Host).
			SetPort(rabbitmqConfig.StreamPort).
			SetUser(rabbitmqConfig.Username).
			SetPassword(rabbitmqConfig.Password).
			SetAddressResolver(stream.AddressResolver{Host: rabbitmqConfig.Host, Port: rabbitmqConfig.StreamPort}),
	)
	if err != nil {
		return event.Handler{}, fmt.Errorf("failed to connect with RabbitMQ stream client: %v", err)
	}
	logger.InfoContext(ctx, "Connected with RabbitMQ stream client", "host", rabbitmqConfig.Host, "port", rabbitmqConfig.StreamPort)

	streamName := "events"
	err = env.DeclareStream(streamName,
		stream.NewStreamOptions().
			SetMaxSegmentSizeBytes(stream.ByteCapacity{}.MB(1)).
			SetMaxAge(1*time.Hour))
	if err != nil {
		return event.Handler{}, fmt.Errorf("failed to declare RabbitMQ stream %q: %v", streamName, err)
	}

	return event.NewHandler(logger, env, streamName), nil
}

type groupService interface {
	FindOrCreate(ctx context.Context, name string, namespace string, hostname string, deployable bool) (*model.Group, error)
}

func createGroups(ctx context.Context, logger *slog.Logger, groupService groupService) error {
	groupNames, err := requireEnvAsArray("GROUP_NAMES")
	if err != nil {
		return err
	}

	groupNamespaces, err := requireEnvAsArray("GROUP_NAMESPACES")
	if err != nil {
		return err
	}
	groupHostnames, err := requireEnvAsArray("GROUP_HOSTNAMES")
	if err != nil {
		return err
	}
	if len(groupNames) != len(groupHostnames) {
		return fmt.Errorf("want arrays to be of equal size, instead got \"GROUP_NAMES\"=%v \"GROUP_HOSTNAMES\"=%v", groupNames, groupHostnames)
	}

	groups := make([]struct{ Name, Namespaces, Hostname string }, len(groupNames))
	for i := 0; i < len(groupNames); i++ {
		groups[i].Name = groupNames[i]
		groups[i].Namespaces = groupNamespaces[i]
		groups[i].Hostname = groupHostnames[i]
	}

	logger.InfoContext(ctx, "Creating groups...")
	for _, g := range groups {
		newGroup, err := groupService.FindOrCreate(ctx, g.Name, g.Namespaces, g.Hostname, true)
		if err != nil {
			return fmt.Errorf("error creating group: %v", err)
		}
		if newGroup != nil {
			logger.InfoContext(ctx, "Created group", "group", newGroup.Name)
		}
	}

	return nil
}

func createAdminUser(ctx context.Context, userService *user.Service, groupService *group.Service) error {
	adminEmail, err := requireEnv("ADMIN_USER_EMAIL")
	if err != nil {
		return err
	}
	adminPassword, err := requireEnv("ADMIN_USER_PASSWORD")
	if err != nil {
		return err
	}

	return user.CreateUser(ctx, adminEmail, adminPassword, userService, groupService, model.AdministratorGroupName, "", "admin")
}

func createE2ETestUser(ctx context.Context, userService *user.Service, groupService *group.Service) error {
	testEmail, err := requireEnv("E2E_TEST_USER_EMAIL")
	if err != nil {
		return err
	}
	testPassword, err := requireEnv("E2E_TEST_USER_PASSWORD")
	if err != nil {
		return err
	}

	return user.CreateUser(ctx, testEmail, testPassword, userService, groupService, model.DefaultGroupName, model.DefaultGroupName, "e2e test")
}

func newGinEngine(logger *slog.Logger) (*gin.Engine, error) {
	// TODO: This is a hack! Allowed origins for different environments should be applied using skaffold profiles... But I can't get it working!
	allowedOrigins, err := requireEnvAsArray("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return nil, err
	}
	basePath, err := requireEnv("BASE_PATH")
	if err != nil {
		return nil, err
	}
	environment, err := requireEnv("ENVIRONMENT")
	if err != nil {
		return nil, err
	}
	if environment != "production" {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://localhost:5173")
	}

	r, err := server.GetEngine(logger, basePath, allowedOrigins)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Gin engine: %v", err)
	}

	return r, nil
}

func requireEnv(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("required environment variable %q not set", key)
	}
	return value, nil
}

func requireEnvAsUint(key string) (uint, error) {
	valueStr, err := requireEnv(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %v", key, err)
	}
	if value > math.MaxUint {
		return 0, fmt.Errorf("value of environment variable %q = %d exceeds uint max value %d", key, value, uint64(math.MaxUint))
	}

	return uint(value), nil
}

func requireEnvAsInt(key string) (int, error) {
	valueStr, err := requireEnv(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %v", key, err)
	}

	return value, nil
}

func requireEnvAsArray(key string) ([]string, error) {
	value, err := requireEnv(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(value, ","), nil
}

func initTracer() (func(), error) {
	host, err := requireEnv("JAEGER_HOST")
	if err != nil {
		return nil, err
	}
	port, err := requireEnvAsUint("JAEGER_PORT")
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("http://%s:%d/api/traces", host, port)
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(endpoint)))
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %v", err)
	}

	environment, err := requireEnv("ENVIRONMENT")
	if err != nil {
		return nil, err
	}

	resources := trace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String(fmt.Sprintf("%s-api", environment))))
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter), resources)
	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Return shutdown function
	return func() { _ = tracerProvider.Shutdown(context.Background()) }, nil
}
