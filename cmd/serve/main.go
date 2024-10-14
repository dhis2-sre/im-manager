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
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/go-mail/mail"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

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
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/integration"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/dhis2-sre/rabbitmq-client/pkg/rabbitmq"
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

	logger := slog.New(log.New(slog.NewJSONHandler(os.Stdout, nil)))
	postgresConfig, err := config.NewPostgresqlConfig()
	if err != nil {
		return err
	}
	db, err := storage.NewDatabase(logger, postgresConfig)
	if err != nil {
		return fmt.Errorf("failed to setup DB: %v", err)
	}

	userRepository := user.NewRepository(db)
	smtpConfig, err := config.NewSMTP()
	if err != nil {
		return err
	}
	dailer := mail.NewDialer(smtpConfig.Host, smtpConfig.Port, smtpConfig.Username, smtpConfig.Password)

	uiURL, err := config.RequireEnv("UI_URL")
	if err != nil {
		return err
	}
	passwordTokenTTL, err := config.RequireEnvAsUint("PASSWORD_TOKEN_TTL")
	if err != nil {
		return err
	}
	userService := user.NewService(uiURL, passwordTokenTTL, userRepository, dailer)
	authorization := middleware.NewAuthorization(logger, userService)

	redisConfig, err := config.NewRedis()
	if err != nil {
		return err
	}
	redis, err := storage.NewRedis(redisConfig.Host, redisConfig.Port)
	if err != nil {
		return err
	}
	tokenRepository := token.NewRepository(redis)

	authConfig, err := config.NewAuthentication()
	if err != nil {
		return err
	}
	privateKey, err := config.GetPrivateKey(logger)
	if err != nil {
		return err
	}
	tokenService, err := token.NewService(logger, tokenRepository, privateKey, authConfig.AccessTokenExpirationSeconds, authConfig.RefreshTokenSecretKey, authConfig.RefreshTokenExpirationSeconds, authConfig.RefreshTokenRememberMeExpirationSeconds)
	if err != nil {
		return err
	}

	publicKey := privateKey.PublicKey
	hostname, err := config.RequireEnv("HOSTNAME")
	if err != nil {
		return err
	}
	userHandler := user.NewHandler(logger, hostname, authConfig.SameSiteMode, authConfig.AccessTokenExpirationSeconds, authConfig.RefreshTokenExpirationSeconds, authConfig.RefreshTokenRememberMeExpirationSeconds, publicKey, userService, tokenService)

	authentication := middleware.NewAuthentication(publicKey, userService)
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)
	groupHandler := group.NewHandler(groupService)

	stacks, err := stack.New(
		stack.DHIS2DB,
		stack.DHIS2Core,
		stack.DHIS2,
		stack.PgAdmin,
		stack.WhoamiGo,
		stack.IMJobRunner,
	)
	if err != nil {
		return fmt.Errorf("error in stack config: %v", err)
	}

	stackService := stack.NewService(stacks)

	instanceParameterEncryptionKey, err := config.RequireEnv("INSTANCE_PARAMETER_ENCRYPTION_KEY")
	if err != nil {
		return err
	}
	instanceRepository := instance.NewRepository(db, instanceParameterEncryptionKey)
	classification, err := config.RequireEnv("CLASSIFICATION")
	if err != nil {
		return err
	}
	helmfileService := instance.NewHelmfileService(logger, stackService, "./stacks", classification)
	instanceService := instance.NewService(logger, instanceRepository, groupService, stackService, helmfileService)

	dockerHubConfig, err := config.NewDockerHub()
	dockerHubClient := integration.NewDockerHubClient(dockerHubConfig.Username, dockerHubConfig.Password)

	rabbitmqConfig, err := config.NewRabbitMQ()
	if err != nil {
		return err
	}
	consumer, err := rabbitmq.NewConsumer(
		rabbitmqConfig.GetURI(),
		rabbitmq.WithConnectionName(hostname),
		rabbitmq.WithConsumerTagPrefix(hostname),
		rabbitmq.WithLogger(logger.WithGroup("rabbitmq")),
	)
	if err != nil {
		return fmt.Errorf("failed to setup RabbitMQ consumer: %v", err)
	}
	defer consumer.Close()

	ttlDestroyConsumer := instance.NewTTLDestroyConsumer(logger, consumer, instanceService)
	err = ttlDestroyConsumer.Consume()
	if err != nil {
		return err
	}

	stackHandler := stack.NewHandler(stackService)

	defaultTTL, err := config.RequireEnvAsUint("DEFAULT_TTL")
	if err != nil {
		return err
	}
	instanceHandler := instance.NewHandler(groupService, instanceService, defaultTTL)

	s3Region, err := config.RequireEnv("S3_REGION")
	if err != nil {
		return err
	}
	// nolint:staticcheck
	s3Endpoint := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		if endpoint := os.Getenv("S3_ENDPOINT"); endpoint != "" {
			return aws.Endpoint{URL: endpoint}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})
	s3Config, err := newS3Config(s3Region, s3Endpoint)
	if err != nil {
		return fmt.Errorf("failed to setup S3 config: %v", err)
	}
	s3AWSClient := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	s3Bucket, err := config.RequireEnv("S3_BUCKET")
	if err != nil {
		return err
	}

	uploader := manager.NewUploader(s3AWSClient)
	s3Client := storage.NewS3Client(logger, s3AWSClient, uploader)

	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService, databaseRepository)
	databaseHandler := database.NewHandler(logger, databaseService, groupService, instanceService, stackService)

	err = handler.RegisterValidation()
	if err != nil {
		return err
	}

	instanceServiceHost, err := config.RequireEnv("INSTANCE_SERVICE_HOST")
	if err != nil {
		return err
	}
	integrationHandler := integration.NewHandler(dockerHubClient, instanceServiceHost)

	logger.Info("Connecting with RabbitMQ stream client", "host", rabbitmqConfig.Host, "port", rabbitmqConfig.StreamPort)
	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetHost(rabbitmqConfig.Host).
			SetPort(rabbitmqConfig.StreamPort).
			SetUser(rabbitmqConfig.Username).
			SetPassword(rabbitmqConfig.Password).
			SetAddressResolver(stream.AddressResolver{Host: rabbitmqConfig.Host, Port: rabbitmqConfig.StreamPort}),
	)
	if err != nil {
		return fmt.Errorf("failed to connect with RabbitMQ stream client: %v", err)
	}
	logger.Info("Connected with RabbitMQ stream client", "host", rabbitmqConfig.Host, "port", rabbitmqConfig.StreamPort)
	streamName := "events"
	err = env.DeclareStream(streamName,
		stream.NewStreamOptions().
			SetMaxSegmentSizeBytes(stream.ByteCapacity{}.MB(1)).
			SetMaxAge(1*time.Hour))
	if err != nil {
		return fmt.Errorf("failed to declare RabbitMQ stream %q: %v", streamName, err)
	}
	eventHandler := event.NewHandler(logger, env, streamName)

	admin, err := config.NewAdminUser()
	if err != nil {
		return err
	}
	err = user.CreateUser(admin.Email, admin.Password, userService, groupService, model.AdministratorGroupName, "admin")
	if err != nil {
		return err
	}
	groups, err := config.NewGroups()
	if err != nil {
		return err
	}
	err = createGroups(logger, groupService, groups)
	if err != nil {
		return err
	}
	testUser, err := config.NewE2eTestUser()
	if err != nil {
		return err
	}
	err = user.CreateUser(testUser.Email, testUser.Password, userService, groupService, model.DefaultGroupName, "e2e test")
	if err != nil {
		return err
	}

	// TODO: This is a hack! Allowed origins for different environments should be applied using skaffold profiles... But I can't get it working!
	allowedOrigins, err := config.RequireEnvAsArray("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return err
	}
	basePath, err := config.RequireEnv("BASE_PATH")
	if err != nil {
		return err
	}
	environment, err := config.RequireEnv("ENVIRONMENT")
	if err != nil {
		return err
	}
	if environment != "production" {
		allowedOrigins = append(allowedOrigins, "http://localhost:3000", "http://localhost:5173")
	}
	r, err := server.GetEngine(logger, basePath, allowedOrigins)
	if err != nil {
		return err
	}

	group.Routes(r, authentication, authorization, groupHandler)
	user.Routes(r, authentication, authorization, userHandler)
	stack.Routes(r, authentication.TokenAuthentication, stackHandler)
	integration.Routes(r, authentication, integrationHandler)
	database.Routes(r, authentication.TokenAuthentication, databaseHandler)
	instance.Routes(r, authentication.TokenAuthentication, instanceHandler)
	event.Routes(r, authentication.TokenAuthentication, eventHandler)

	logger.Info("Listening and serving HTTP")
	if err := r.Run(); err != nil {
		return fmt.Errorf("failed to start the HTTP server: %v", err)
	}
	return nil
}

type groupService interface {
	FindOrCreate(name string, hostname string, deployable bool) (*model.Group, error)
}

func createGroups(logger *slog.Logger, groupService groupService, groups []config.Group) error {
	logger.Info("Creating groups...")
	for _, g := range groups {
		newGroup, err := groupService.FindOrCreate(g.Name, g.Hostname, true)
		if err != nil {
			return fmt.Errorf("error creating group: %v", err)
		}
		if newGroup != nil {
			logger.Info("Created group", "group", newGroup.Name)
		}
	}

	return nil
}

func newS3Config(region string, endpoint aws.EndpointResolverWithOptionsFunc) (aws.Config, error) {
	config, err := s3config.LoadDefaultConfig(
		context.TODO(),
		s3config.WithRegion(region),
		s3config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptions(endpoint)),
	)
	if err != nil {
		return aws.Config{}, err
	}

	return config, nil
}
