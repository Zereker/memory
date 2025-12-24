package mq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// Package-level singleton instance
var producerInstance *KafkaProducer

// Init initializes the Kafka producer singleton with config.
func Init(cfg KafkaConfig) error {
	producer, err := NewKafkaProducer(cfg)
	if err != nil {
		return err
	}
	producerInstance = producer
	return nil
}

// NewQueue returns the singleton Kafka producer instance.
// Returns nil if Kafka is not enabled or not initialized.
func NewQueue() *KafkaProducer {
	return producerInstance
}

// KafkaConfig Kafka 配置
type KafkaConfig struct {
	Brokers   []string         `toml:"brokers"`
	Consumers []ConsumerConfig `toml:"consumers"`
	Enabled   bool             `toml:"enabled"`
}

// ConsumerConfig 单个消费者配置
type ConsumerConfig struct {
	Name   string   `toml:"name"`   // 消费者名称（用于日志）
	Group  string   `toml:"group"`  // 消费组
	Topics []string `toml:"topics"` // 订阅的 topics
}

// Validate 验证配置
func (c *KafkaConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if len(c.Brokers) == 0 {
		return fmt.Errorf("brokers is required when kafka is enabled")
	}
	for i, consumer := range c.Consumers {
		if consumer.Group == "" {
			return fmt.Errorf("consumers[%d].group is required", i)
		}
		if len(consumer.Topics) == 0 {
			return fmt.Errorf("consumers[%d].topics is required", i)
		}
	}
	return nil
}

// MessageHandler 消息处理函数
type MessageHandler func(ctx context.Context, topic string, message []byte) error

// KafkaConsumer Kafka 消费者
type KafkaConsumer struct {
	logger  *slog.Logger
	name    string
	topics  []string
	client  sarama.ConsumerGroup
	handler MessageHandler
	ready   chan struct{}
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewKafkaConsumer 创建 Kafka 消费者
func NewKafkaConsumer(brokers []string, config ConsumerConfig, handler MessageHandler) (*KafkaConsumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup(brokers, config.Group, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	name := config.Name
	if name == "" {
		name = config.Group
	}

	return &KafkaConsumer{
		logger:  slog.Default().With("module", "kafka-consumer", "name", name),
		name:    name,
		topics:  config.Topics,
		client:  client,
		handler: handler,
		ready:   make(chan struct{}),
	}, nil
}

// Start 启动消费者
func (c *KafkaConsumer) Start(ctx context.Context) error {
	if c == nil {
		return nil
	}

	ctx, c.cancel = context.WithCancel(ctx)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			handler := &consumerGroupHandler{
				ready:   c.ready,
				handler: c.handler,
				logger:  c.logger,
			}

			if err := c.client.Consume(ctx, c.topics, handler); err != nil {
				if err == context.Canceled {
					return
				}
				c.logger.Error("consumer error", "error", err)
				time.Sleep(time.Second)
			}

			if ctx.Err() != nil {
				return
			}

			c.ready = make(chan struct{})
		}
	}()

	// 等待消费者就绪
	<-c.ready
	c.logger.Info("consumer started", "topics", c.topics)

	return nil
}

// Stop 停止消费者
func (c *KafkaConsumer) Stop() error {
	if c == nil {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()

	if c.client != nil {
		return c.client.Close()
	}

	return nil
}

// consumerGroupHandler 实现 sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	ready   chan struct{}
	handler MessageHandler
	logger  *slog.Logger
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			h.logger.Debug("received message",
				"topic", message.Topic,
				"partition", message.Partition,
				"offset", message.Offset,
			)

			if err := h.handler(session.Context(), message.Topic, message.Value); err != nil {
				h.logger.Error("failed to handle message",
					"topic", message.Topic,
					"error", err,
				)
				// 继续处理下一条消息，不阻塞
			}

			session.MarkMessage(message, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// KafkaProducer Kafka 生产者
type KafkaProducer struct {
	logger *slog.Logger
	config KafkaConfig
	client sarama.SyncProducer
}

// 确保 KafkaProducer 实现 MessageQueue 接口
var _ MessageQueue = (*KafkaProducer)(nil)

// NewKafkaProducer 创建 Kafka 生产者
func NewKafkaProducer(config KafkaConfig) (*KafkaProducer, error) {
	if !config.Enabled {
		return nil, nil
	}

	saramaConfig := sarama.NewConfig()
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	saramaConfig.Producer.Retry.Max = 3

	client, err := sarama.NewSyncProducer(config.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	return &KafkaProducer{
		logger: slog.Default().With("module", "kafka-producer"),
		config: config,
		client: client,
	}, nil
}

// Publish 发布消息
func (p *KafkaProducer) Publish(topic string, message []byte) error {
	if p == nil {
		return nil
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(message),
	}

	partition, offset, err := p.client.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	p.logger.Debug("message sent",
		"topic", topic,
		"partition", partition,
		"offset", offset,
	)

	return nil
}

// Close 关闭生产者
func (p *KafkaProducer) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}

// Subscribe 订阅（Producer 不支持，仅用于满足 MessageQueue 接口）
func (p *KafkaProducer) Subscribe(topic string, handler func(message []byte) error) error {
	return fmt.Errorf("kafka producer does not support subscribe, use KafkaConsumer instead")
}
