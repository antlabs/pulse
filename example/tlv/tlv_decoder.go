package main

import (
	"encoding/binary"
	"errors"
	"fmt"
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

// TLVDecoder TLV格式解码器
type TLVDecoder struct {
	buffer []byte // 缓冲区，用于处理不完整的数据包
}

// NewTLVDecoder 创建新的TLV解码器
func NewTLVDecoder() *TLVDecoder {
	return &TLVDecoder{
		buffer: make([]byte, 0, 1024),
	}
}

// Decode 解码TLV格式的数据
// 返回值：解码后的TLVMessage，剩余的字节数据，错误信息
func (d *TLVDecoder) Decode(data []byte) (*TLVMessage, []byte, error) {
	// 将新数据追加到缓冲区
	d.buffer = append(d.buffer, data...)

	// TLV最小长度：Type(2字节) + Length(4字节) = 6字节
	if len(d.buffer) < 6 {
		return nil, nil, errors.New("insufficient data for TLV header")
	}

	// 解析Type字段（2字节，大端序）
	msgType := binary.BigEndian.Uint16(d.buffer[0:2])

	// 解析Length字段（4字节，大端序）
	msgLength := binary.BigEndian.Uint32(d.buffer[2:6])

	// 检查数据长度是否合理（防止恶意数据）
	if msgLength > 1024*1024 { // 限制最大1MB
		return nil, nil, errors.New("message length too large")
	}

	// 检查是否有足够的数据
	totalLength := 6 + int(msgLength)
	if len(d.buffer) < totalLength {
		return nil, nil, errors.New("insufficient data for complete TLV message")
	}

	// 提取Value数据
	value := make([]byte, msgLength)
	copy(value, d.buffer[6:6+msgLength])

	// 创建TLV消息
	tlvMsg := &TLVMessage{
		Type:   msgType,
		Length: msgLength,
		Value:  value,
	}

	// 计算剩余数据
	remaining := make([]byte, len(d.buffer)-totalLength)
	copy(remaining, d.buffer[totalLength:])

	// 更新缓冲区
	d.buffer = d.buffer[:0]
	if len(remaining) > 0 {
		d.buffer = append(d.buffer, remaining...)
	}

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

// DecodeStateless 无状态解码函数，不依赖解码器实例的buffer
// 参数：buffer是上次剩余的数据，newData是新接收到的数据
// 返回值：解码后的TLVMessage，更新后的buffer，错误信息
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
