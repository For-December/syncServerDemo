package transport

import (
	"fmt"
	"sync"
)

// LocalTransport 本地内存实现的传输层，用于测试和演示
type LocalTransport struct {
	channels map[string]chan Message // 每个客户端的消息通道
	incoming chan MessageWithSender  // 服务器接收通道
	mu       sync.RWMutex
	closed   bool
}

type MessageWithSender struct {
	ClientID string
	Message  Message
}

// NewLocalTransport 创建本地传输层
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{
		channels: make(map[string]chan Message),
		incoming: make(chan MessageWithSender, 100),
		closed:   false,
	}
}

func (t *LocalTransport) Register(clientID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	if _, exists := t.channels[clientID]; exists {
		return fmt.Errorf("client %s already registered", clientID)
	}

	t.channels[clientID] = make(chan Message, 100)
	return nil
}

func (t *LocalTransport) Unregister(clientID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if ch, exists := t.channels[clientID]; exists {
		close(ch)
		delete(t.channels, clientID)
	}
	return nil
}

func (t *LocalTransport) Send(clientID string, msg Message) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	ch, exists := t.channels[clientID]
	if !exists {
		return fmt.Errorf("client %s not found", clientID)
	}

	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("client %s channel full", clientID)
	}
}

func (t *LocalTransport) Broadcast(msg Message, excludeID string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	for id, ch := range t.channels {
		if id == excludeID {
			continue
		}
		select {
		case ch <- msg:
		default:
			// 如果通道满了，跳过这个客户端
		}
	}
	return nil
}

func (t *LocalTransport) Receive() (string, Message, error) {
	msg, ok := <-t.incoming
	if !ok {
		return "", nil, fmt.Errorf("transport closed")
	}
	return msg.ClientID, msg.Message, nil
}

func (t *LocalTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	for _, ch := range t.channels {
		close(ch)
	}
	close(t.incoming)
	return nil
}

// GetClientChannel 获取客户端的接收通道（用于客户端读取消息）
func (t *LocalTransport) GetClientChannel(clientID string) (chan Message, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ch, exists := t.channels[clientID]
	if !exists {
		return nil, fmt.Errorf("client %s not found", clientID)
	}
	return ch, nil
}

// SendToServer 客户端发送消息到服务器
func (t *LocalTransport) SendToServer(clientID string, msg Message) error {
	if t.closed {
		return fmt.Errorf("transport is closed")
	}

	select {
	case t.incoming <- MessageWithSender{ClientID: clientID, Message: msg}:
		return nil
	default:
		return fmt.Errorf("server incoming channel full")
	}
}
