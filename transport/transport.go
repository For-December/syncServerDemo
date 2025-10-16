package transport

// Message 网络消息接口
type Message interface {
	GetType() string
	GetData() interface{}
}

// Transport 网络传输抽象接口，可以替换为TCP、UDP、WebSocket等实现
type Transport interface {
	// Send 发送消息到指定客户端
	Send(clientID string, msg Message) error

	// Broadcast 广播消息到所有客户端（排除指定ID）
	Broadcast(msg Message, excludeID string) error

	// Receive 接收消息（阻塞）
	Receive() (clientID string, msg Message, err error)

	// Register 注册客户端
	Register(clientID string) error

	// Unregister 注销客户端
	Unregister(clientID string) error

	// Close 关闭传输层
	Close() error
}

// BaseMessage 基础消息结构
type BaseMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func (m *BaseMessage) GetType() string {
	return m.Type
}

func (m *BaseMessage) GetData() interface{} {
	return m.Data
}

// NewMessage 创建消息
func NewMessage(msgType string, data interface{}) Message {
	return &BaseMessage{
		Type: msgType,
		Data: data,
	}
}
