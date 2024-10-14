package config

import (
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
)

func NewPostgresqlConfig() (Postgresql, error) {
	host, err := RequireEnv("DATABASE_HOST")
	if err != nil {
		return Postgresql{}, err
	}
	port, err := requireEnvAsInt("DATABASE_PORT")
	if err != nil {
		return Postgresql{}, err
	}
	username, err := RequireEnv("DATABASE_USERNAME")
	if err != nil {
		return Postgresql{}, err
	}
	password, err := RequireEnv("DATABASE_PASSWORD")
	if err != nil {
		return Postgresql{}, err
	}
	name, err := RequireEnv("DATABASE_NAME")
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
	host, err := RequireEnv("SMTP_HOST")
	if err != nil {
		return SMTP{}, err
	}
	port, err := requireEnvAsInt("SMTP_PORT")
	if err != nil {
		return SMTP{}, err
	}
	username, err := RequireEnv("SMTP_USERNAME")
	if err != nil {
		return SMTP{}, err
	}
	password, err := RequireEnv("SMTP_PASSWORD")
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
	host, err := RequireEnv("REDIS_HOST")
	if err != nil {
		return Redis{}, err
	}
	port, err := requireEnvAsInt("REDIS_PORT")
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
	refreshTokenSecretKey, err := RequireEnv("REFRESH_TOKEN_SECRET_KEY")
	if err != nil {
		return Authentication{}, err
	}
	accessTokenExpirationSeconds, err := requireEnvAsInt("ACCESS_TOKEN_EXPIRATION_IN_SECONDS")
	if err != nil {
		return Authentication{}, err
	}
	refreshTokenExpirationSeconds, err := requireEnvAsInt("REFRESH_TOKEN_EXPIRATION_IN_SECONDS")
	if err != nil {
		return Authentication{}, err
	}
	refreshTokenRememberMeExpirationSeconds, err := requireEnvAsInt("REFRESH_TOKEN_REMEMBER_ME_EXPIRATION_IN_SECONDS")
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
	sameSiteMode, err := RequireEnv("SAME_SITE_MODE")
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
	username, err := RequireEnv("DOCKER_HUB_USERNAME")
	if err != nil {
		return DockerHub{}, err
	}
	password, err := RequireEnv("DOCKER_HUB_PASSWORD")
	if err != nil {
		return DockerHub{}, err
	}

	return DockerHub{
		Username: username,
		Password: password,
	}, nil
}

func NewRabbitMQ() (RabbitMQ, error) {
	host, err := RequireEnv("RABBITMQ_HOST")
	if err != nil {
		return RabbitMQ{}, err
	}
	port, err := requireEnvAsInt("RABBITMQ_PORT")
	if err != nil {
		return RabbitMQ{}, err
	}
	streamPort, err := requireEnvAsInt("RABBITMQ_STREAM_PORT")
	if err != nil {
		return RabbitMQ{}, err
	}
	username, err := RequireEnv("RABBITMQ_USERNAME")
	if err != nil {
		return RabbitMQ{}, err
	}
	password, err := RequireEnv("RABBITMQ_PASSWORD")
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
	key, err := RequireEnv("PRIVATE_KEY")
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
	groupNames, err := RequireEnvAsArray("GROUP_NAMES")
	if err != nil {
		return nil, err
	}
	groupHostnames, err := RequireEnvAsArray("GROUP_HOSTNAMES")
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
	email, err := RequireEnv("ADMIN_USER_EMAIL")
	if err != nil {
		return user{}, err
	}
	password, err := RequireEnv("ADMIN_USER_PASSWORD")
	if err != nil {
		return user{}, err
	}

	return user{
		Email:    email,
		Password: password,
	}, nil
}

func NewE2eTestUser() (user, error) {
	email, err := RequireEnv("E2E_TEST_USER_EMAIL")
	if err != nil {
		return user{}, err
	}
	password, err := RequireEnv("E2E_TEST_USER_PASSWORD")
	if err != nil {
		return user{}, err
	}

	return user{
		Email:    email,
		Password: password,
	}, nil
}

func RequireEnv(key string) (string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return "", fmt.Errorf("required environment variable %q not set", key)
	}
	return value, nil
}

func requireEnvAsInt(key string) (int, error) {
	valueStr, err := RequireEnv(key)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse environment variable %q as int: %v", key, err)
	}

	return value, nil
}

func RequireEnvAsUint(key string) (uint, error) {
	valueStr, err := RequireEnv(key)
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

func RequireEnvAsArray(key string) ([]string, error) {
	value, err := RequireEnv(key)
	if err != nil {
		return nil, err
	}
	return strings.Split(value, ","), nil
}
