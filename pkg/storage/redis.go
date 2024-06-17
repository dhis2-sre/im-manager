package storage

import (
	"fmt"

	"github.com/go-redis/redis"
)

func NewRedis(host string, port int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: "",
		DB:       0,
	})

	if _, err := client.Ping().Result(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %v", err)
	}

	return client, nil
}
