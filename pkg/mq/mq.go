package mq

// MessageQueue 消息队列接口
type MessageQueue interface {
	Publish(topic string, message []byte) error
	Subscribe(topic string, handler func(message []byte) error) error
	Close() error
}
