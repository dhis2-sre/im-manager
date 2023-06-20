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
	"log"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	"github.com/dhis2-sre/rabbitmq"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.New()

	db, err := storage.NewDatabase(cfg.Postgresql)
	if err != nil {
		return err
	}

	userRepository := user.NewRepository(db)
	userService := user.NewService(userRepository)
	authorization := middleware.NewAuthorization(userService)
	redis := storage.NewRedis(cfg)
	tokenRepository := token.NewRepository(redis)
	privateKey, err := cfg.Authentication.Keys.GetPrivateKey()
	if err != nil {
		return err
	}
	publicKey, err := cfg.Authentication.Keys.GetPublicKey()
	if err != nil {
		return err
	}
	tokenService, err := token.NewService(tokenRepository, privateKey, publicKey, cfg.Authentication.AccessTokenExpirationSeconds, cfg.Authentication.RefreshTokenSecretKey, cfg.Authentication.RefreshTokenExpirationSeconds)
	if err != nil {
		return err
	}

	userHandler := user.NewHandler(userService, tokenService)

	authentication := middleware.NewAuthentication(publicKey, userService)
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)
	groupHandler := group.NewHandler(groupService)

	stackService := stack.NewService(stack.NewRepository(db))

	instanceRepo := instance.NewRepository(db, cfg)
	helmfileService := instance.NewHelmfileService(stackService, cfg)
	instanceService := instance.NewService(cfg, instanceRepo, groupService, stackService, helmfileService)

	dockerHubClient := integration.NewDockerHubClient(cfg.DockerHub.Username, cfg.DockerHub.Password)

	err = stack.LoadStacks("./stacks", stackService)
	if err != nil {
		return err
	}

	consumer, err := rabbitmq.NewConsumer(
		cfg.RabbitMqURL.GetUrl(),
		rabbitmq.WithConsumerPrefix("im-manager"),
	)
	if err != nil {
		return err
	}
	defer consumer.Close()

	ttlDestroyConsumer := instance.NewTTLDestroyConsumer(consumer, instanceService)
	err = ttlDestroyConsumer.Consume()
	if err != nil {
		return err
	}

	stackHandler := stack.NewHandler(stackService)
	instanceHandler := instance.NewHandler(userService, groupService, instanceService, stackService, cfg.DefaultTTL)

	// TODO: Database... Move into... Function?
	s3Config, err := s3config.LoadDefaultConfig(context.TODO(), s3config.WithRegion("eu-west-1"))
	if err != nil {
		return err
	}
	s3AWSClient := s3.NewFromConfig(s3Config)
	uploader := manager.NewUploader(s3AWSClient)
	s3Client := storage.NewS3Client(s3AWSClient, uploader)

	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(cfg.Bucket, s3Client, groupService, databaseRepository)
	databaseHandler := database.NewHandler(databaseService, groupService, instanceService, stackService)

	err = handler.RegisterValidation()
	if err != nil {
		return err
	}

	integrationHandler := integration.NewHandler(dockerHubClient, cfg.InstanceService.Host, cfg.DatabaseManagerService.Host)

	err = createAdminUser(cfg, userService, groupService)
	if err != nil {
		return err
	}
	err = createGroups(cfg, groupService)
	if err != nil {
		return err
	}

	r := server.GetEngine(cfg.BasePath)

	group.Routes(r, authentication, authorization, groupHandler)
	user.Routes(r, authentication, authorization, userHandler)
	stack.Routes(r, authentication.TokenAuthentication, stackHandler)
	integration.Routes(r, authentication, integrationHandler)
	database.Routes(r, authentication.TokenAuthentication, databaseHandler)
	instance.Routes(r, authentication, instanceHandler)

	return r.Run()
}

type groupService interface {
	FindOrCreate(name string, hostname string, deployable bool) (*model.Group, error)
	AddUser(groupName string, userId uint) error
}

type userService interface {
	FindOrCreate(email string, password string) (*model.User, error)
}

func createGroups(config config.Config, groupService groupService) error {
	log.Println("Creating groups...")
	groups := config.Groups
	for _, g := range groups {
		newGroup, err := groupService.FindOrCreate(g.Name, g.Hostname, true)
		if err != nil {
			return fmt.Errorf("error creating group: %v", err)
		}
		if newGroup != nil {
			log.Println("Created:", newGroup.Name)
		}
	}

	return nil
}

func createAdminUser(config config.Config, userService userService, groupService groupService) error {
	adminUserEmail := config.AdminUser.Email
	adminUserPassword := config.AdminUser.Password

	u, err := userService.FindOrCreate(adminUserEmail, adminUserPassword)
	if err != nil {
		return fmt.Errorf("error creating admin user: %v", err)
	}

	g, err := groupService.FindOrCreate(model.AdministratorGroupName, "", false)
	if err != nil {
		return fmt.Errorf("error creating admin group: %v", err)
	}

	err = groupService.AddUser(g.Name, u.ID)
	if err != nil {
		return fmt.Errorf("error adding admin user to admin group: %v", err)
	}

	return nil
}
