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
	InstanceParameterEncryptionKey string
	InstanceService                Service
	S3Bucket                       string
	S3Region                       string
	S3Endpoint                     string
}

func New() Config {
	return Config{
		Environment: requireEnv("ENVIRONMENT"),
		InstanceService: Service{
			Host: requireEnv("INSTANCE_SERVICE_HOST"),
		},
		S3Bucket:   requireEnv("S3_BUCKET"),
		S3Region:   requireEnv("S3_REGION"),
		S3Endpoint: os.Getenv("S3_ENDPOINT"),
	}
}

func NewPostgresqlConfig() (Postgresql, error) {
	host, err := RequireEnvNew("DATABASE_HOST")
	if err != nil {
		return Postgresql{}, err
	}
	port, err := requireEnvNewAsInt("DATABASE_PORT")
	if err != nil {
		return Postgresql{}, err
	}
	username, err := RequireEnvNew("DATABASE_USERNAME")
	if err != nil {
		return Postgresql{}, err
	}
	password, err := RequireEnvNew("DATABASE_PASSWORD")
	if err != nil {
		return Postgresql{}, err
	}
	name, err := RequireEnvNew("DATABASE_NAME")
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
	host, err := RequireEnvNew("SMTP_HOST")
	if err != nil {
		return SMTP{}, err
	}
	port, err := requireEnvNewAsInt("SMTP_PORT")
	if err != nil {
		return SMTP{}, err
	}
	username, err := RequireEnvNew("SMTP_USERNAME")
	if err != nil {
		return SMTP{}, err
	}
	password, err := RequireEnvNew("SMTP_PASSWORD")
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

func NewRedis() (Redis, error) {
	host, err := RequireEnvNew("REDIS_HOST")
	if err != nil {
		return Redis{}, err
	}
	port, err := requireEnvNewAsInt("REDIS_PORT")
	if err != nil {
		return Redis{}, err
	}
	return Redis{
		Host: host,
		Port: port,
	}, nil
}

func NewAuthentication() (Authentication, error) {
	mode, err := sameSiteMode()
	if err != nil {
		return Authentication{}, err
	}
	refreshTokenSecretKey, err := RequireEnvNew("REFRESH_TOKEN_SECRET_KEY")
	if err != nil {
		return Authentication{}, err
	}
	accessTokenExpirationSeconds, err := requireEnvNewAsInt("ACCESS_TOKEN_EXPIRATION_IN_SECONDS")
	if err != nil {
		return Authentication{}, err
	}
	refreshTokenExpirationSeconds, err := requireEnvNewAsInt("REFRESH_TOKEN_EXPIRATION_IN_SECONDS")
	if err != nil {
		return Authentication{}, err
	}
	refreshTokenRememberMeExpirationSeconds, err := requireEnvNewAsInt("REFRESH_TOKEN_REMEMBER_ME_EXPIRATION_IN_SECONDS")
	if err != nil {
		return Authentication{}, err
	}

	return Authentication{
		SameSiteMode:                            mode,
		RefreshTokenSecretKey:                   refreshTokenSecretKey,
		AccessTokenExpirationSeconds:            accessTokenExpirationSeconds,
		RefreshTokenExpirationSeconds:           refreshTokenExpirationSeconds,
		RefreshTokenRememberMeExpirationSeconds: refreshTokenRememberMeExpirationSeconds,
	}, nil
}

func sameSiteMode() (http.SameSite, error) {
	sameSiteMode, err := RequireEnvNew("SAME_SITE_MODE")
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

func NewDockerHub() (DockerHub, error) {
	username, err := RequireEnvNew("DOCKER_HUB_USERNAME")
	if err != nil {
		return DockerHub{}, err
	}
	password, err := RequireEnvNew("DOCKER_HUB_PASSWORD")
	if err != nil {
		return DockerHub{}, err
	}

	return DockerHub{
		Username: username,
		Password: password,
	}, nil
}

func NewRabbitMQ() (RabbitMQ, error) {
	host, err := RequireEnvNew("RABBITMQ_HOST")
	if err != nil {
		return RabbitMQ{}, err
	}
	port, err := requireEnvNewAsInt("RABBITMQ_PORT")
	if err != nil {
		return RabbitMQ{}, err
	}
	streamPort, err := requireEnvNewAsInt("RABBITMQ_STREAM_PORT")
	if err != nil {
		return RabbitMQ{}, err
	}
	username, err := RequireEnvNew("RABBITMQ_USERNAME")
	if err != nil {
		return RabbitMQ{}, err
	}
	password, err := RequireEnvNew("RABBITMQ_PASSWORD")
	if err != nil {
		return RabbitMQ{}, err
	}

	return RabbitMQ{
		Host:       host,
		Port:       port,
		StreamPort: streamPort,
		Username:   username,
		Password:   password,
	}, nil
}

type Service struct {
	Host string
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

type RabbitMQ struct {
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
func (r RabbitMQ) GetURI() string {
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
	key, err := RequireEnvNew("PRIVATE_KEY")
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
	groupNames, err := RequireEnvNewAsArray("GROUP_NAMES")
	if err != nil {
		return nil, err
	}
	groupHostnames, err := RequireEnvNewAsArray("GROUP_HOSTNAMES")
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
	email, err := RequireEnvNew("ADMIN_USER_EMAIL")
	if err != nil {
		return user{}, err
	}
	password, err := RequireEnvNew("ADMIN_USER_PASSWORD")
	if err != nil {
		return user{}, err
	}

	return user{
		Email:    email,
		Password: password,
	}, nil
}

func NewE2eTestUser() (user, error) {
	email, err := RequireEnvNew("E2E_TEST_USER_EMAIL")
	if err != nil {
		return user{}, err
	}
	password, err := RequireEnvNew("E2E_TEST_USER_PASSWORD")
	if err != nil {
		return user{}, err
	}

	return user{
		Email:    email,
		Password: password,
	}, nil
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

func RequireEnvNew(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("required environment variable %q not set", key)
	}
	return value, nil
}

func requireEnvNewAsInt(key string) (int, error) {
	valueStr, err := RequireEnvNew(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %v", key, err)
	}

	return value, nil
}

func RequireEnvNewAsUint(key string) (uint, error) {
	valueStr, err := RequireEnvNew(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %v", key, err)
	}

	return uint(value), nil
}

func RequireEnvNewAsArray(key string) ([]string, error) {
	value, err := RequireEnvNew(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(value, ","), nil
}
