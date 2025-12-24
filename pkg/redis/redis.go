package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Package-level singleton instance
var clientInstance *redis.Client

// Config Redis 配置
type Config struct {
	Addr     string `toml:"addr"`
	Password string `toml:"password"`
	DB       int    `toml:"db"`
	Enabled  bool   `toml:"enabled"`
}

// Validate 验证配置
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Addr == "" {
		return fmt.Errorf("addr is required when redis is enabled")
	}
	return nil
}

// Init initializes the Redis client singleton with config.
func Init(cfg Config) error {
	if !cfg.Enabled {
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}

	clientInstance = client
	return nil
}

// Client returns the singleton Redis client instance.
// Returns nil if Redis is not enabled or not initialized.
func Client() *redis.Client {
	return clientInstance
}

// Close closes the Redis client connection.
func Close() error {
	if clientInstance == nil {
		return nil
	}
	return clientInstance.Close()
}
