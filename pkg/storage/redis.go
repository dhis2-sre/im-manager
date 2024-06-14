package storage

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/config"

	"github.com/go-redis/redis"
)

func NewRedis(cfg config.Redis) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: "",
		DB:       0,
	})

	if _, err := client.Ping().Result(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %v", err)
	}

	return client, nil
}
