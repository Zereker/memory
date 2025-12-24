package consumer

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/Zereker/memory/internal/action"
	"github.com/Zereker/memory/pkg/mq"
)

// Consumer 异步任务消费者
// TODO: 重新设计用于 Zep 风格的异步处理（如 Community 检测等）
type Consumer struct {
	logger    *slog.Logger
	memory    *action.Memory
	consumers []*mq.KafkaConsumer
}

// Config 消费者配置
type Config struct {
	Kafka mq.KafkaConfig
}

// NewConsumer 创建消费者
func NewConsumer(memory *action.Memory, cfg Config) (*Consumer, error) {
	c := &Consumer{
		logger: slog.Default().With("module", "consumer"),
		memory: memory,
	}

	if !cfg.Kafka.Enabled {
		c.logger.Info("kafka disabled, consumer not started")
		return c, nil
	}

	// TODO: 重新配置消费者用于 Zep 风格处理
	c.logger.Info("kafka consumer placeholder - to be implemented for Zep architecture")

	return c, nil
}

// Start 启动所有消费者
func (c *Consumer) Start(ctx context.Context) error {
	if len(c.consumers) == 0 {
		c.logger.Info("no consumers configured, skipping start")
		return nil
	}

	c.logger.Info("starting consumers", "count", len(c.consumers))

	g, ctx := errgroup.WithContext(ctx)
	for _, consumer := range c.consumers {
		consumer := consumer
		g.Go(func() error {
			return consumer.Start(ctx)
		})
	}

	return g.Wait()
}

// Stop 停止所有消费者
func (c *Consumer) Stop() error {
	c.logger.Info("stopping consumers")

	for _, consumer := range c.consumers {
		if err := consumer.Stop(); err != nil {
			c.logger.Error("failed to stop consumer", "error", err)
		}
	}

	return nil
}
