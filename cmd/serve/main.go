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

	"github.com/go-mail/mail"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/internal/log"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/database"
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
		fmt.Printf("im-manager exits due to: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.New()

	logger := slog.New(log.New(slog.NewJSONHandler(os.Stdout, nil)))
	db, err := storage.NewDatabase(logger, cfg.Postgresql)
	if err != nil {
		return err
	}

	userRepository := user.NewRepository(db)
	dailer := mail.NewDialer(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.Username, cfg.SMTP.Password)
	userService := user.NewService(cfg.UIURL, cfg.PasswordTokenTTL, userRepository, dailer)
	authorization := middleware.NewAuthorization(logger, userService)
	redis, err := storage.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	tokenRepository := token.NewRepository(redis)
	authConfig := cfg.Authentication
	privateKey, err := authConfig.Keys.GetPrivateKey(logger)
	if err != nil {
		return err
	}
	publicKey, err := authConfig.Keys.GetPublicKey()
	if err != nil {
		return err
	}
	tokenService, err := token.NewService(logger, tokenRepository, privateKey, publicKey, authConfig.AccessTokenExpirationSeconds, authConfig.RefreshTokenSecretKey, authConfig.RefreshTokenExpirationSeconds, authConfig.RefreshTokenRememberMeExpirationSeconds)
	if err != nil {
		return err
	}

	// TODO: Assert authConfig.SameSiteMode not -1
	userHandler := user.NewHandler(logger, cfg.Hostname, authConfig.SameSiteMode, authConfig.AccessTokenExpirationSeconds, authConfig.RefreshTokenExpirationSeconds, authConfig.RefreshTokenRememberMeExpirationSeconds, publicKey, userService, tokenService)

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

	instanceRepository := instance.NewRepository(db, cfg.InstanceParameterEncryptionKey)
	helmfileService := instance.NewHelmfileService(logger, stackService, "./stacks", cfg.Classification)
	instanceService := instance.NewService(logger, instanceRepository, groupService, stackService, helmfileService)

	dockerHubClient := integration.NewDockerHubClient(cfg.DockerHub.Username, cfg.DockerHub.Password)

	host := hostname()
	consumer, err := rabbitmq.NewConsumer(
		cfg.RabbitMqURL.GetUrl(),
		rabbitmq.WithConnectionName(host),
		rabbitmq.WithConsumerTagPrefix(host),
		rabbitmq.WithLogger(logger.WithGroup("rabbitmq")),
	)
	if err != nil {
		return err
	}
	defer consumer.Close()

	ttlDestroyConsumer := instance.NewTTLDestroyConsumer(logger, consumer, instanceService)
	err = ttlDestroyConsumer.Consume()
	if err != nil {
		return err
	}

	stackHandler := stack.NewHandler(stackService)
	instanceHandler := instance.NewHandler(groupService, instanceService, cfg.DefaultTTL)

	s3Endpoint := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		if cfg.S3Endpoint != "" {
			return aws.Endpoint{URL: cfg.S3Endpoint}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	s3Config, err := newS3Config(cfg.S3Region, s3Endpoint)
	if err != nil {
		return err
	}

	s3AWSClient := s3.NewFromConfig(s3Config, func(o *s3.Options) {
		o.UsePathStyle = true
	})
	uploader := manager.NewUploader(s3AWSClient)
	s3Client := storage.NewS3Client(logger, s3AWSClient, uploader)

	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, cfg.S3Bucket, s3Client, groupService, databaseRepository)
	databaseHandler := database.NewHandler(logger, databaseService, groupService, instanceService, stackService)

	err = handler.RegisterValidation()
	if err != nil {
		return err
	}

	integrationHandler := integration.NewHandler(dockerHubClient, cfg.InstanceService.Host, cfg.DatabaseManagerService.Host)

	err = user.CreateUser(cfg.AdminUser.Email, cfg.AdminUser.Password, userService, groupService, model.AdministratorGroupName, "admin")
	if err != nil {
		return err
	}
	err = createGroups(logger, groupService, cfg.Groups)
	if err != nil {
		return err
	}
	err = user.CreateUser(cfg.E2eTestUser.Email, cfg.E2eTestUser.Password, userService, groupService, model.DefaultGroupName, "e2e test")
	if err != nil {
		return err
	}

	// TODO: This is a hack! Allowed origins for different environments should be applied using skaffold profiles... But I can't get it working!
	if cfg.Environment != "production" {
		cfg.AllowedOrigins = append(cfg.AllowedOrigins, "http://localhost:3000", "http://localhost:5173")
	}
	r := server.GetEngine(logger, cfg.BasePath, cfg.AllowedOrigins)

	group.Routes(r, authentication, authorization, groupHandler)
	user.Routes(r, authentication, authorization, userHandler)
	stack.Routes(r, authentication.TokenAuthentication, stackHandler)
	integration.Routes(r, authentication, integrationHandler)
	database.Routes(r, authentication.TokenAuthentication, databaseHandler)
	instance.Routes(r, authentication.TokenAuthentication, instanceHandler)

	return r.Run()
}

func hostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "im-manager"
	}
	return hostname
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
