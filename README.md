# pulse
极简异步io库，速度很快，非常简单

## 特性

- 🚀 **高性能**：基于epoll/kqueue的事件驱动架构
- 🎯 **极简API**：只需实现OnOpen、OnData、OnClose三个回调
- 🔄 **多任务模式**：支持事件循环、协程池、独占协程三种处理模式
- 🛡️ **并发安全**：内置连接管理和状态隔离
- 🌐 **跨平台**：支持Linux、macOS，Windows开发中

## 支持平台

* Linux
* macOS  
* Windows (TODO - 开发中)

## 安装

```bash
go get github.com/antlabs/pulse
```

## 快速开始

### Echo服务器

```go
package main

import (
    "context"
    "log"
    
    "github.com/antlabs/pulse"
)

type EchoHandler struct{}

func (h *EchoHandler) OnOpen(c *pulse.Conn, err error) {
    if err != nil {
        log.Printf("连接失败: %v", err)
        return
    }
    log.Println("客户端连接成功")
}

func (h *EchoHandler) OnData(c *pulse.Conn, data []byte) {
    // 回显收到的数据
    c.Write(data)
}

func (h *EchoHandler) OnClose(c *pulse.Conn, err error) {
    log.Println("连接关闭")
}

func main() {
    server, err := pulse.NewMultiEventLoop(
        context.Background(),
        pulse.WithCallback(&EchoHandler{}),
        pulse.WithTaskType(pulse.TaskTypeInEventLoop),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    log.Println("服务器启动在 :8080")
    server.ListenAndServe(":8080")
}
```

### 自定义协议解析

```go
package main

import (
    "context"
    "log"
    
    "github.com/antlabs/pulse"
)

type ProtocolHandler struct{}

func (h *ProtocolHandler) OnOpen(c *pulse.Conn, err error) {
    if err != nil {
        return
    }
    // 为每个连接初始化解析状态
    c.SetSession(make([]byte, 0))
}

func (h *ProtocolHandler) OnData(c *pulse.Conn, data []byte) {
    // 获取连接的解析缓冲区
    buffer := c.GetSession().([]byte)
    
    // 将新数据追加到缓冲区
    buffer = append(buffer, data...)
    
    // 解析完整消息
    for len(buffer) >= 4 { // 假设消息长度前缀为4字节
        msgLen := int(buffer[0])<<24 | int(buffer[1])<<16 | int(buffer[2])<<8 | int(buffer[3])
        if len(buffer) < 4+msgLen {
            break // 消息不完整，等待更多数据
        }
        
        // 提取完整消息
        message := buffer[4 : 4+msgLen]
        log.Printf("收到消息: %s", string(message))
        
        // 处理消息...
        
        // 移除已处理的消息
        buffer = buffer[4+msgLen:]
    }
    
    // 更新连接的缓冲区
    c.SetSession(buffer)
}

func (h *ProtocolHandler) OnClose(c *pulse.Conn, err error) {
    log.Println("连接关闭")
}

func main() {
    server, err := pulse.NewMultiEventLoop(
        context.Background(),
        pulse.WithCallback(&ProtocolHandler{}),
        pulse.WithTaskType(pulse.TaskTypeInEventLoop),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    server.ListenAndServe(":8080")
}
```

## 主要概念

### 回调接口

```go
type Callback[T any] interface {
    OnOpen(c *Conn, err error)  // 连接建立时调用
    OnData(c *Conn, data T)     // 接收到数据时调用  
    OnClose(c *Conn, err error) // 连接关闭时调用
}
```

### 任务处理模式

```go
// 在事件循环中处理（推荐，性能最好, redis和nginx场景）
pulse.WithTaskType(pulse.TaskTypeInEventLoop)

// 在协程池中处理（适合CPU密集型任务, 常见业务场景）
pulse.WithTaskType(pulse.TaskTypeInBusinessGoroutine)

// 每个连接独占一个协程（适合阻塞IO）
pulse.WithTaskType(pulse.TaskTypeInConnectionGoroutine)
```

### 连接管理

```go
type Conn struct{}

// 写入数据
func (c *Conn) Write(data []byte) (int, error)

// 关闭连接
func (c *Conn) Close()

// 设置会话数据（用于存储连接状态）
func (c *Conn) SetSession(session any)

// 获取会话数据
func (c *Conn) GetSession() any

// 设置超时
func (c *Conn) SetReadDeadline(t time.Time) error
func (c *Conn) SetWriteDeadline(t time.Time) error
```

## 配置选项

```go
server, err := pulse.NewMultiEventLoop(
    context.Background(),
    pulse.WithCallback(&handler{}),                    // 设置回调处理器
    pulse.WithTaskType(pulse.TaskTypeInEventLoop),     // 设置任务处理模式
    pulse.WithTriggerType(pulse.TriggerTypeEdge),      // 设置触发模式（边缘/水平）
    pulse.WithEventLoopReadBufferSize(4096),           // 设置读缓冲区大小
    pulse.WithLogLevel(slog.LevelInfo),                // 设置日志级别
)
```

## 示例项目

- [Echo服务器](example/echo/server/server.go) - 基础回显服务器
- [TLV协议解析](example/tlv/) - 完整的TLV协议解析示例
- [Core API使用](example/core/) - 底层API使用示例

## 性能测试

```bash
# 启动echo服务器
cd example/echo/server && go run server.go

# 使用wrk进行压测
wrk -t12 -c400 -d30s --script=lua/echo.lua http://127.0.0.1:8080
```

## 最佳实践

1. **状态管理**：使用 `SetSession/GetSession` 存储连接级别的状态
2. **协议解析**：推荐使用无状态解析函数，避免全局共享状态
3. **错误处理**：在OnOpen和OnClose中正确处理错误
4. **内存管理**：及时释放大的临时缓冲区，使用连接池复用连接
5. **并发安全**：避免在多个goroutine中同时操作同一个连接

## 架构设计

```
应用层 ┌─────────────────────────────────────┐
      │  OnOpen / OnData / OnClose          │
      └─────────────────────────────────────┘
框架层 ┌─────────────────────────────────────┐
      │  Connection Management              │
      │  Task Scheduling                    │  
      │  Event Loop                         │
      └─────────────────────────────────────┘
系统层 ┌─────────────────────────────────────┐
      │  epoll (Linux) / kqueue (macOS)     │
      └─────────────────────────────────────┘
```

## 贡献

欢迎提交Issue和Pull Request！

## 许可证

Apache License 2.0
