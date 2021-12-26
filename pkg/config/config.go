package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

func ProvideConfig() Config {
	return Config{
		BasePath: requireEnv("BASE_PATH"),
		UserService: service{
			Host:     requireEnv("USER_SERVICE_HOST"),
			BasePath: requireEnv("USER_SERVICE_BASE_PATH"),
			Username: requireEnv("USER_SERVICE_USERNAME"),
			Password: requireEnv("USER_SERVICE_PASSWORD"),
		},
		Postgresql: postgresql{
			Host:         requireEnv("DATABASE_HOST"),
			Port:         requireEnvAsInt("DATABASE_PORT"),
			Username:     requireEnv("DATABASE_USERNAME"),
			Password:     requireEnv("DATABASE_PASSWORD"),
			DatabaseName: requireEnv("DATABASE_NAME"),
		},
		RabbitMqURL: rabbitmq{
			Host:     requireEnv("RABBITMQ_HOST"),
			Port:     requireEnvAsInt("RABBITMQ_PORT"),
			Username: requireEnv("RABBITMQ_USERNAME"),
			Password: requireEnv("RABBITMQ_PASSWORD"),
		},
		Authentication: Authentication{
			Jwks: Jwks{
				Host:                   requireEnv("JWKS_HOST"),
				Index:                  requireEnvAsInt("JWKS_INDEX"),
				MinimumRefreshInterval: requireEnvAsInt("JWKS_MINIMUM_REFRESH_INTERVAL"),
			},
		},
	}
}

type Config struct {
	BasePath       string
	UserService    service
	Postgresql     postgresql
	RabbitMqURL    rabbitmq
	Authentication Authentication
}

type service struct {
	Host     string
	BasePath string
	Username string
	Password string
}

type postgresql struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
}

type rabbitmq struct {
	Host     string
	Port     int
	Username string
	Password string
}

func (r rabbitmq) GetUrl() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", r.Username, r.Password, r.Host, r.Port)
}

type Authentication struct {
	Jwks Jwks
}

type Jwks struct {
	Host                   string
	Index                  int
	MinimumRefreshInterval int
}

func requireEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Can't find environment varialbe: %s\n", key)
	}
	return value
}

func requireEnvAsInt(key string) int {
	valueStr := requireEnv(key)
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Fatalf("Can't parse value as integer: %s", err.Error())
	}
	return value
}
