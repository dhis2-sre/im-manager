package database

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	BasePath       string
	UserService    databseService
	Postgresql     postgresql
	RabbitMqURL    rabbitmq
	Authentication Authentication
	Bucket         string
}

func NewConfig() (Config, error) {
	basePath, err := requireEnv("BASE_PATH")
	if err != nil {
		return Config{}, err
	}

	usrSvc, err := newUserService()
	if err != nil {
		return Config{}, err
	}

	pg, err := newPostgresql()
	if err != nil {
		return Config{}, err
	}

	rb, err := newRabbitMQ()
	if err != nil {
		return Config{}, err
	}

	auth, err := newAuthentication()
	if err != nil {
		return Config{}, err
	}

	bucket, err := requireEnv("S3_BUCKET")
	if err != nil {
		return Config{}, err
	}

	return Config{
		BasePath:       basePath,
		UserService:    usrSvc,
		Postgresql:     pg,
		RabbitMqURL:    rb,
		Authentication: auth,
		Bucket:         bucket,
	}, nil
}

type databseService struct {
	Host     string
	BasePath string
	Username string
	Password string
}

func newUserService() (databseService, error) {
	host, err := requireEnv("USER_SERVICE_HOST")
	if err != nil {
		return databseService{}, err
	}
	basePath, err := requireEnv("USER_SERVICE_BASE_PATH")
	if err != nil {
		return databseService{}, err
	}

	return databseService{
		Host:     host,
		BasePath: basePath,
	}, nil
}

type postgresql struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
}

func newPostgresql() (postgresql, error) {
	host, err := requireEnv("DATABASE_HOST")
	if err != nil {
		return postgresql{}, err
	}
	port, err := requireEnvAsInt("DATABASE_PORT")
	if err != nil {
		return postgresql{}, err
	}
	usrname, err := requireEnv("DATABASE_USERNAME")
	if err != nil {
		return postgresql{}, err
	}
	pw, err := requireEnv("DATABASE_PASSWORD")
	if err != nil {
		return postgresql{}, err
	}
	name, err := requireEnv("DATABASE_NAME")
	if err != nil {
		return postgresql{}, err
	}

	return postgresql{
		Host:         host,
		Port:         port,
		Username:     usrname,
		Password:     pw,
		DatabaseName: name,
	}, nil
}

type rabbitmq struct {
	Host     string
	Port     int
	Username string
	Password string
}

func newRabbitMQ() (rabbitmq, error) {
	host, err := requireEnv("RABBITMQ_HOST")
	if err != nil {
		return rabbitmq{}, err
	}
	port, err := requireEnvAsInt("RABBITMQ_PORT")
	if err != nil {
		return rabbitmq{}, err
	}
	username, err := requireEnv("RABBITMQ_USERNAME")
	if err != nil {
		return rabbitmq{}, err
	}
	pw, err := requireEnv("RABBITMQ_PASSWORD")
	if err != nil {
		return rabbitmq{}, err
	}

	return rabbitmq{
		Host:     host,
		Port:     port,
		Username: username,
		Password: pw,
	}, nil
}

func (r rabbitmq) GetUrl() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", r.Username, r.Password, r.Host, r.Port)
}

type Authentication struct {
	Jwks Jwks
}

func newAuthentication() (Authentication, error) {
	host, err := requireEnv("JWKS_HOST")
	if err != nil {
		return Authentication{}, err
	}
	index, err := requireEnvAsInt("JWKS_INDEX")
	if err != nil {
		return Authentication{}, err
	}
	refreshInterval, err := requireEnvAsInt("JWKS_MINIMUM_REFRESH_INTERVAL")
	if err != nil {
		return Authentication{}, err
	}

	return Authentication{
		Jwks: Jwks{
			Host:                   host,
			Index:                  index,
			MinimumRefreshInterval: refreshInterval,
		},
	}, nil
}

type Jwks struct {
	Host                   string
	Index                  int
	MinimumRefreshInterval int
}

func requireEnv(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("can't find environment variable: %s", key)
	}
	return value, nil
}

func requireEnvAsInt(key string) (int, error) {
	valueStr, err := requireEnv(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("can't parse value as integer: %v", err)
	}
	return value, nil
}
