package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Environment                    string
	Classification                 string
	Hostname                       string
	UIURL                          string
	AllowedOrigins                 []string
	InstanceParameterEncryptionKey string
	BasePath                       string
	DefaultTTL                     uint
	InstanceService                Service
	DockerHub                      DockerHub
	DatabaseManagerService         Service
	Postgresql                     Postgresql
	RabbitMqURL                    rabbitmq
	SMTP                           smtp
	Redis                          redis
	Authentication                 Authentication
	Groups                         []group
	AdminUser                      user
	E2eTestUser                    user
	S3Bucket                       string
	S3Region                       string
	S3Endpoint                     string
}

func New() Config {
	return Config{
		Environment:                    requireEnv("ENVIRONMENT"),
		Classification:                 requireEnv("CLASSIFICATION"),
		Hostname:                       requireEnv("HOSTNAME"),
		UIURL:                          requireEnv("UI_URL"),
		AllowedOrigins:                 requireEnvAsArray("CORS_ALLOWED_ORIGINS"),
		BasePath:                       requireEnv("BASE_PATH"),
		InstanceParameterEncryptionKey: requireEnv("INSTANCE_PARAMETER_ENCRYPTION_KEY"),
		DefaultTTL:                     uint(requireEnvAsInt("DEFAULT_TTL")),
		InstanceService: Service{
			Host:     requireEnv("INSTANCE_SERVICE_HOST"),
			BasePath: requireEnv("INSTANCE_SERVICE_BASE_PATH"),
		},
		DockerHub: DockerHub{
			Username: requireEnv("DOCKER_HUB_USERNAME"),
			Password: requireEnv("DOCKER_HUB_PASSWORD"),
		},
		DatabaseManagerService: Service{
			Host:     requireEnv("DATABASE_MANAGER_SERVICE_HOST"),
			BasePath: requireEnv("DATABASE_MANAGER_SERVICE_BASE_PATH"),
			//			Username: requireEnv("DATABASE_MANAGER_SERVICE_USERNAME"),
			//			Password: requireEnv("DATABASE_MANAGER_SERVICE_PASSWORD"),
		},
		Postgresql: Postgresql{
			Host:         requireEnv("DATABASE_HOST"),
			Port:         requireEnvAsInt("DATABASE_PORT"),
			Username:     requireEnv("DATABASE_USERNAME"),
			Password:     requireEnv("DATABASE_PASSWORD"),
			DatabaseName: requireEnv("DATABASE_NAME"),
		},
		SMTP: smtp{
			Host:     requireEnv("SMTP_HOST"),
			Port:     requireEnvAsInt("SMTP_PORT"),
			Username: requireEnv("SMTP_USERNAME"),
			Password: requireEnv("SMTP_PASSWORD"),
		},
		RabbitMqURL: rabbitmq{
			Host:     requireEnv("RABBITMQ_HOST"),
			Port:     requireEnvAsInt("RABBITMQ_PORT"),
			Username: requireEnv("RABBITMQ_USERNAME"),
			Password: requireEnv("RABBITMQ_PASSWORD"),
		},
		Redis: redis{
			Host: requireEnv("REDIS_HOST"),
			Port: requireEnvAsInt("REDIS_PORT"),
		},
		Authentication: Authentication{
			Keys: keys{
				PrivateKey: requireEnv("PRIVATE_KEY"),
				PublicKey:  requireEnv("PUBLIC_KEY"),
			},
			RefreshTokenSecretKey:         requireEnv("REFRESH_TOKEN_SECRET_KEY"),
			AccessTokenExpirationSeconds:  requireEnvAsInt("ACCESS_TOKEN_EXPIRATION_IN_SECONDS"),
			RefreshTokenExpirationSeconds: requireEnvAsInt("REFRESH_TOKEN_EXPIRATION_IN_SECONDS"),
		},
		Groups:      newGroups(),
		AdminUser:   newAdminUser(),
		E2eTestUser: newE2eTestUser(),
		S3Bucket:    requireEnv("S3_BUCKET"),
		S3Region:    requireEnv("S3_REGION"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
	}
}

type Service struct {
	Host     string
	BasePath string
	Username string
	Password string
}

type DockerHub struct {
	Username string
	Password string
}

type Postgresql struct {
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

type smtp struct {
	Host     string
	Port     int
	Username string
	Password string
}

func (r rabbitmq) GetUrl() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", r.Username, r.Password, r.Host, r.Port)
}

type redis struct {
	Host string
	Port int
}

type Authentication struct {
	Keys                          keys
	RefreshTokenSecretKey         string
	AccessTokenExpirationSeconds  int
	RefreshTokenExpirationSeconds int
}

type keys struct {
	PrivateKey string
	PublicKey  string
}

func (k keys) GetPrivateKey() (*rsa.PrivateKey, error) {
	decode, _ := pem.Decode([]byte(k.PrivateKey))
	if decode == nil {
		return nil, errors.New("failed to decode private key")
	}

	// Openssl generates keys formatted as PKCS8 but terraform tls_private_key is producing PKCS1
	// So if parsing of PKCS8 fails we try PKCS1
	privateKey, err := x509.ParsePKCS8PrivateKey(decode.Bytes)
	if err != nil {
		if err.Error() == "x509: failed to parse private key (use ParsePKCS1PrivateKey instead for this key format)" {
			log.Println("Trying to parse PKCS1 format...")
			privateKey, err = x509.ParsePKCS1PrivateKey(decode.Bytes)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		log.Println("Successfully parsed private key")
	}

	return privateKey.(*rsa.PrivateKey), nil
}

func (k keys) GetPublicKey() (*rsa.PublicKey, error) {
	decode, _ := pem.Decode([]byte(k.PublicKey))
	if decode == nil {
		return nil, errors.New("failed to decode public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(decode.Bytes)
	if err != nil {
		return nil, err
	}

	return publicKey.(*rsa.PublicKey), nil
}

type group struct {
	Name     string
	Hostname string
}

func newGroups() []group {
	groupNames := requireEnvAsArray("GROUP_NAMES")
	groupHostnames := requireEnvAsArray("GROUP_HOSTNAMES")

	if len(groupNames) != len(groupHostnames) {
		log.Fatalf("len(GROUP_NAMES) != len(GROUP_HOSTNAMES)")
	}

	groups := make([]group, len(groupNames))
	for i := 0; i < len(groupNames); i++ {
		groups[i].Name = groupNames[i]
		groups[i].Hostname = groupHostnames[i]
	}

	return groups
}

type user struct {
	Email    string
	Password string
}

func newAdminUser() user {
	email := requireEnv("ADMIN_USER_EMAIL")
	pw := requireEnv("ADMIN_USER_PASSWORD")

	return user{
		Email:    email,
		Password: pw,
	}
}

func newE2eTestUser() user {
	email := requireEnv("E2E_TEST_USER_EMAIL")
	pw := requireEnv("E2E_TEST_USER_PASSWORD")

	return user{
		Email:    email,
		Password: pw,
	}
}

func requireEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Can't find environment variable: %s\n", key)
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

func requireEnvAsArray(key string) []string {
	value := requireEnv(key)
	return strings.Split(value, ",")
}
