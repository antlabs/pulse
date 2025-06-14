package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

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

// DecodeTLV 解码TLV消息
func DecodeTLV(data []byte) (uint16, uint32, []byte, error) {
	if len(data) < 6 {
		return 0, 0, nil, fmt.Errorf("insufficient data for TLV header")
	}

	msgType := binary.BigEndian.Uint16(data[0:2])
	msgLength := binary.BigEndian.Uint32(data[2:6])

	if len(data) < 6+int(msgLength) {
		return 0, 0, nil, fmt.Errorf("insufficient data for complete TLV message")
	}

	value := make([]byte, msgLength)
	copy(value, data[6:6+msgLength])

	return msgType, msgLength, value, nil
}

func main() {
	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	defer conn.Close()

	fmt.Println("Connected to TLV server")

	// 发送Echo消息
	echoData := []byte("Hello, TLV Server!")
	echoMessage := EncodeTLV(1, echoData)
	_, err = conn.Write(echoMessage)
	if err != nil {
		log.Fatal("Failed to send echo message:", err)
	}
	fmt.Printf("Sent Echo message: %s\n", string(echoData))

	// 读取Echo响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil && err != io.EOF {
		log.Fatal("Failed to read echo response:", err)
	}

	if n > 0 {
		msgType, msgLength, value, err := DecodeTLV(buffer[:n])
		if err != nil {
			log.Printf("Failed to decode echo response: %v", err)
		} else {
			fmt.Printf("Received Echo response - Type: %d, Length: %d, Value: %s\n",
				msgType, msgLength, string(value))
		}
	}

	// 等待一秒
	time.Sleep(time.Second)

	// 发送Ping消息
	pingData := []byte("ping")
	pingMessage := EncodeTLV(2, pingData)
	_, err = conn.Write(pingMessage)
	if err != nil {
		log.Fatal("Failed to send ping message:", err)
	}
	fmt.Printf("Sent Ping message: %s\n", string(pingData))

	// 读取Pong响应
	n, err = conn.Read(buffer)
	if err != nil && err != io.EOF {
		log.Fatal("Failed to read pong response:", err)
	}

	if n > 0 {
		msgType, msgLength, value, err := DecodeTLV(buffer[:n])
		if err != nil {
			log.Printf("Failed to decode pong response: %v", err)
		} else {
			fmt.Printf("Received Pong response - Type: %d, Length: %d, Value: %s\n",
				msgType, msgLength, string(value))
		}
	}

	// 等待一秒
	time.Sleep(time.Second)

	// 发送未知类型消息
	unknownData := []byte("unknown message")
	unknownMessage := EncodeTLV(99, unknownData)
	_, err = conn.Write(unknownMessage)
	if err != nil {
		log.Fatal("Failed to send unknown message:", err)
	}
	fmt.Printf("Sent Unknown message: %s\n", string(unknownData))

	// 读取未知类型响应
	n, err = conn.Read(buffer)
	if err != nil && err != io.EOF {
		log.Fatal("Failed to read unknown response:", err)
	}

	if n > 0 {
		msgType, msgLength, value, err := DecodeTLV(buffer[:n])
		if err != nil {
			log.Printf("Failed to decode unknown response: %v", err)
		} else {
			fmt.Printf("Received Unknown response - Type: %d, Length: %d, Value: %s\n",
				msgType, msgLength, string(value))
		}
	}

	fmt.Println("Client finished")
}
