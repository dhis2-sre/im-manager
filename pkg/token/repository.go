package token

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(redisClient *redis.Client) *redisTokenRepository {
	return &redisTokenRepository{
		redis: redisClient,
	}
}

type redisTokenRepository struct {
	redis *redis.Client
}

func (r redisTokenRepository) SetRefreshToken(userId uint, tokenId string, expiresIn time.Duration) error {
	key := fmt.Sprintf("%d:%s", userId, tokenId)
	if err := r.redis.Set(key, 0, expiresIn).Err(); err != nil {
		return fmt.Errorf("could not SET refresh token to redis for userId/tokenId: %d/%s: %s", userId, tokenId, err)
	}
	return nil
}

func (r redisTokenRepository) DeleteRefreshToken(userId uint, previousTokenId string) error {
	key := fmt.Sprintf("%d:%s", userId, previousTokenId)

	result := r.redis.Del(key)

	if err := result.Err(); err != nil {
		return fmt.Errorf("could not delete refresh token to redis for userId/tokenId: %d/%s: %s", userId, previousTokenId, err)
	}

	if result.Val() < 1 {
		return fmt.Errorf("refresh token to redis for userId/tokenId does not exist: %d/%s", userId, previousTokenId)
	}

	return nil
}

func (r redisTokenRepository) DeleteRefreshTokens(userId uint) error {
	pattern := fmt.Sprintf("%d*", userId)

	iterator := r.redis.Scan(0, pattern, 5).Iterator()
	failCount := 0

	for iterator.Next() {
		if err := r.redis.Del(iterator.Val()).Err(); err != nil {
			failCount++
		}
	}

	if err := iterator.Err(); err != nil {
		return fmt.Errorf("failed to delete refresh token: %s", iterator.Val())
	}

	if failCount > 0 {
		return fmt.Errorf("failed to delete refresh token: %s", iterator.Val())
	}

	return nil
}
