package mq

// InMemoryQueue 内存消息队列（用于测试和简单场景）
type InMemoryQueue struct {
	handlers map[string][]func([]byte) error
	messages map[string][][]byte
}

// 确保 InMemoryQueue 实现 MessageQueue 接口
var _ MessageQueue = (*InMemoryQueue)(nil)

// NewInMemoryQueue 创建内存消息队列
func NewInMemoryQueue() *InMemoryQueue {
	return &InMemoryQueue{
		handlers: make(map[string][]func([]byte) error),
		messages: make(map[string][][]byte),
	}
}

// Publish 发布消息（同步处理）
func (q *InMemoryQueue) Publish(topic string, message []byte) error {
	q.messages[topic] = append(q.messages[topic], message)

	// 同步调用所有 handlers
	for _, handler := range q.handlers[topic] {
		if err := handler(message); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe 订阅 topic
func (q *InMemoryQueue) Subscribe(topic string, handler func([]byte) error) error {
	q.handlers[topic] = append(q.handlers[topic], handler)
	return nil
}

// Close 关闭
func (q *InMemoryQueue) Close() error {
	return nil
}

// GetMessages 获取指定 topic 的所有消息（用于测试）
func (q *InMemoryQueue) GetMessages(topic string) [][]byte {
	return q.messages[topic]
}
