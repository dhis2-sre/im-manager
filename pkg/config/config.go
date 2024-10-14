package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
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
	PasswordTokenTTL               uint
	InstanceService                Service
	DockerHub                      DockerHub
	DatabaseManagerService         Service
	RabbitMqURL                    rabbitmq
	Redis                          Redis
	Authentication                 Authentication
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
		PasswordTokenTTL:               uint(requireEnvAsInt("PASSWORD_TOKEN_TTL")),
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
		RabbitMqURL: rabbitmq{
			Host:       requireEnv("RABBITMQ_HOST"),
			Port:       requireEnvAsInt("RABBITMQ_PORT"),
			StreamPort: requireEnvAsInt("RABBITMQ_STREAM_PORT"),
			Username:   requireEnv("RABBITMQ_USERNAME"),
			Password:   requireEnv("RABBITMQ_PASSWORD"),
		},
		Redis: Redis{
			Host: requireEnv("REDIS_HOST"),
			Port: requireEnvAsInt("REDIS_PORT"),
		},
		Authentication: Authentication{
			SameSiteMode:                            sameSiteMode(),
			RefreshTokenSecretKey:                   requireEnv("REFRESH_TOKEN_SECRET_KEY"),
			AccessTokenExpirationSeconds:            requireEnvAsInt("ACCESS_TOKEN_EXPIRATION_IN_SECONDS"),
			RefreshTokenExpirationSeconds:           requireEnvAsInt("REFRESH_TOKEN_EXPIRATION_IN_SECONDS"),
			RefreshTokenRememberMeExpirationSeconds: requireEnvAsInt("REFRESH_TOKEN_REMEMBER_ME_EXPIRATION_IN_SECONDS"),
		},
		E2eTestUser: newE2eTestUser(),
		S3Bucket:    requireEnv("S3_BUCKET"),
		S3Region:    requireEnv("S3_REGION"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
	}
}

func NewPostgresqlConfig() (Postgresql, error) {
	host, err := requireEnvNew("DATABASE_HOST")
	if err != nil {
		return Postgresql{}, err
	}
	port, err := requireEnvNewAsInt("DATABASE_PORT")
	if err != nil {
		return Postgresql{}, err
	}
	username, err := requireEnvNew("DATABASE_USERNAME")
	if err != nil {
		return Postgresql{}, err
	}
	password, err := requireEnvNew("DATABASE_PASSWORD")
	if err != nil {
		return Postgresql{}, err
	}
	name, err := requireEnvNew("DATABASE_NAME")
	if err != nil {
		return Postgresql{}, err
	}

	return Postgresql{
			Host:         host,
			Port:         port,
			Username:     username,
			Password:     password,
			DatabaseName: name,
		},
		nil
}

func NewSMTP() (SMTP, error) {
	host, err := requireEnvNew("SMTP_HOST")
	if err != nil {
		return SMTP{}, err
	}
	port, err := requireEnvNewAsInt("SMTP_PORT")
	if err != nil {
		return SMTP{}, err
	}
	username, err := requireEnvNew("SMTP_USERNAME")
	if err != nil {
		return SMTP{}, err
	}
	password, err := requireEnvNew("SMTP_PASSWORD")
	if err != nil {
		return SMTP{}, err
	}

	return SMTP{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	}, nil
}

func sameSiteMode() http.SameSite {
	sameSiteMode := requireEnv("SAME_SITE_MODE")
	switch sameSiteMode {
	case "default":
		return http.SameSiteDefaultMode
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	}

	log.Fatalf("Can't parse same site mode: %s\n", sameSiteMode)
	return -1
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
	Host       string
	Port       int
	StreamPort int
	Username   string
	Password   string
}

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
}

// GetURI returns the AMQP URI for RabbitMQ.
func (r rabbitmq) GetURI() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d/", r.Username, r.Password, r.Host, r.Port)
}

type Redis struct {
	Host string
	Port int
}

type Authentication struct {
	SameSiteMode                            http.SameSite
	RefreshTokenSecretKey                   string
	AccessTokenExpirationSeconds            int
	RefreshTokenExpirationSeconds           int
	RefreshTokenRememberMeExpirationSeconds int
}

func GetPrivateKey(logger *slog.Logger) (*rsa.PrivateKey, error) {
	key, err := requireEnvNew("PRIVATE_KEY")
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
			logger.Info("Trying to parse PKCS1 format...")
			privateKey, err = x509.ParsePKCS1PrivateKey(decode.Bytes)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		logger.Info("Successfully parsed private key")
	}

	return privateKey.(*rsa.PrivateKey), nil
}

type Group struct {
	Name     string
	Hostname string
}

func NewGroups() ([]Group, error) {
	groupNames, err := requireEnvNewAsArray("GROUP_NAMES")
	if err != nil {
		return nil, err
	}
	groupHostnames, err := requireEnvNewAsArray("GROUP_HOSTNAMES")
	if err != nil {
		return nil, err
	}
	if len(groupNames) != len(groupHostnames) {
		return nil, fmt.Errorf("want arrays to be of equal size, instead got \"GROUP_NAMES\"=%v \"GROUP_HOSTNAMES\"=%v", groupNames, groupHostnames)
	}

	groups := make([]Group, len(groupNames))
	for i := 0; i < len(groupNames); i++ {
		groups[i].Name = groupNames[i]
		groups[i].Hostname = groupHostnames[i]
	}

	return groups, nil
}

type user struct {
	Email    string
	Password string
}

func NewAdminUser() (user, error) {
	email, err := requireEnvNew("ADMIN_USER_EMAIL")
	if err != nil {
		return user{}, err
	}
	password, err := requireEnvNew("ADMIN_USER_PASSWORD")
	if err != nil {
		return user{}, err
	}

	return user{
		Email:    email,
		Password: password,
	}, nil
}

func newE2eTestUser() user {
	email := requireEnv("E2E_TEST_USER_EMAIL")
	pw := requireEnv("E2E_TEST_USER_PASSWORD")

	return user{
		Email:    email,
		Password: pw,
	}
}

// Deprecated: requiredEnv is deprecated. Use requiredEnvNew instead.
// TODO(DEVOPS-394) replace this function with requiredEnvNew, renaming requiredEnvNew to
// requiredEnv
func requireEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("Can't find environment variable: %s\n", key)
	}
	return value
}

func requireEnvNew(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("required environment variable %q not set", key)
	}
	return value, nil
}

func requireEnvNewAsInt(key string) (int, error) {
	valueStr, err := requireEnvNew(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %v", key, err)
	}

	return value, nil
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

func requireEnvNewAsArray(key string) ([]string, error) {
	value, err := requireEnvNew(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(value, ","), nil
}
