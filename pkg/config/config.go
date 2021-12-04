package config

import (
	"log"
	"os"
	"strconv"
)

func ProvideConfig() Config {
	return Config{
		BasePath: requireEnv("BASE_PATH"),
		Postgresql: postgresql{
			Host:         requireEnv("DATABASE_HOST"),
			Port:         requireEnvAsInt("DATABASE_PORT"),
			Username:     requireEnv("DATABASE_USERNAME"),
			Password:     requireEnv("DATABASE_PASSWORD"),
			DatabaseName: requireEnv("DATABASE_NAME"),
		},
	}
}

type Config struct {
	BasePath   string
	Postgresql postgresql
}

type postgresql struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
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
