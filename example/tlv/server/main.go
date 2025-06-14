package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"log/slog"

	"github.com/antlabs/pulse"
)

// TLVMessage 表示一个TLV消息
type TLVMessage struct {
	Type   uint16 // 消息类型，2字节
	Length uint32 // 数据长度，4字节
	Value  []byte // 实际数据
}

// String 用于打印TLV消息
func (t *TLVMessage) String() string {
	return fmt.Sprintf("TLV{Type: %d, Length: %d, Value: %s}", t.Type, t.Length, string(t.Value))
}

// DecodeStateless 无状态解码函数，不依赖解码器实例的buffer
func DecodeStateless(buffer, newData []byte) (*TLVMessage, []byte, error) {
	// 将新数据追加到缓冲区
	combined := append(buffer, newData...)

	// TLV最小长度：Type(2字节) + Length(4字节) = 6字节
	if len(combined) < 6 {
		return nil, combined, errors.New("insufficient data for TLV header")
	}

	// 解析Type字段（2字节，大端序）
	msgType := binary.BigEndian.Uint16(combined[0:2])

	// 解析Length字段（4字节，大端序）
	msgLength := binary.BigEndian.Uint32(combined[2:6])

	// 检查数据长度是否合理（防止恶意数据）
	if msgLength > 1024*1024 { // 限制最大1MB
		return nil, nil, errors.New("message length too large")
	}

	// 检查是否有足够的数据
	totalLength := 6 + int(msgLength)
	if len(combined) < totalLength {
		return nil, combined, errors.New("insufficient data for complete TLV message")
	}

	// 提取Value数据
	value := make([]byte, msgLength)
	copy(value, combined[6:6+msgLength])

	// 创建TLV消息
	tlvMsg := &TLVMessage{
		Type:   msgType,
		Length: msgLength,
		Value:  value,
	}

	// 计算剩余数据
	remaining := make([]byte, len(combined)-totalLength)
	copy(remaining, combined[totalLength:])

	return tlvMsg, remaining, nil
}

// EncodeTLV 编码TLV消息为字节数组
func EncodeTLV(msgType uint16, data []byte) []byte {
	msgLength := uint32(len(data))
	result := make([]byte, 6+len(data))

	// 写入Type（2字节，大端序）
	binary.BigEndian.PutUint16(result[0:2], msgType)

	// 写入Length（4字节，大端序）
	binary.BigEndian.PutUint32(result[2:6], msgLength)

	// 写入Value
	copy(result[6:], data)

	return result
}

// TLVCallback 处理TLV消息的回调
type TLVCallback struct{}

func (cb *TLVCallback) OnOpen(c *pulse.Conn, err error) {
	if err != nil {
		slog.Error("Connection failed", "error", err)
		return
	}

	// 为每个连接初始化空的buffer
	c.SetSession(make([]byte, 0))

	slog.Info("Client connected")
}

func (cb *TLVCallback) OnData(c *pulse.Conn, data []byte) {
	// 获取当前连接的buffer
	buffer, ok := c.GetSession().([]byte)
	if !ok {
		slog.Error("Failed to get buffer from session")
		return
	}

	// 使用无状态解码函数进行解码
	tlvMsg, remainingBuffer, err := DecodeStateless(buffer, data)
	if err != nil {
		// 如果是数据不完整的错误，保存buffer等待更多数据
		if err.Error() == "insufficient data for TLV header" ||
			err.Error() == "insufficient data for complete TLV message" {
			c.SetSession(remainingBuffer)
			return
		}
		slog.Error("Failed to decode TLV message", "error", err)
		return
	}

	// 更新连接的buffer为剩余数据
	c.SetSession(remainingBuffer)

	slog.Info("Received TLV message", "message", tlvMsg.String())

	// 根据消息类型处理不同的逻辑
	switch tlvMsg.Type {
	case 1: // Echo消息
		response := EncodeTLV(1, []byte(fmt.Sprintf("Echo: %s", string(tlvMsg.Value))))
		c.Write(response)
	case 2: // Ping消息
		response := EncodeTLV(3, []byte("Pong"))
		c.Write(response)
	default:
		slog.Warn("Unknown message type", "type", tlvMsg.Type)
		response := EncodeTLV(999, []byte("Unknown message type"))
		c.Write(response)
	}
}

func (cb *TLVCallback) OnClose(c *pulse.Conn, err error) {
	if err != nil {
		slog.Info("Connection closed with error", "error", err)
	} else {
		slog.Info("Connection closed")
	}
}

func main() {
	// 配置日志级别
	slog.SetDefault(slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// 创建回调处理器
	callback := &TLVCallback{}

	// 启动多事件循环服务器
	server, err := pulse.NewMultiEventLoop(
		context.Background(),
		pulse.WithCallback(callback),
		pulse.WithTaskType(pulse.TaskTypeInEventLoop),
	)

	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	slog.Info("TLV Server starting", "address", ":8080")

	// 监听并处理连接
	err = server.ListenAndServe(":8080")
	if err != nil {
		log.Fatal("Server failed:", err)
	}
}
