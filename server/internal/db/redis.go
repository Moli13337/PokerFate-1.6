package db

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedis(addr string) (*redis.Client, error) {
	opts, err := redis.ParseURL(addr)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return rdb, nil
}
