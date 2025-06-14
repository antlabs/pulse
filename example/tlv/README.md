# TLV 解码器示例

这个示例展示了如何在 Pulse 框架中实现和使用 TLV（Type-Length-Value）格式的解码器。

## TLV 格式说明

TLV 是一种常见的数据编码格式，由三部分组成：

- **Type (2字节)**：消息类型，使用大端序编码
- **Length (4字节)**：Value 的长度，使用大端序编码  
- **Value (可变长度)**：实际的数据内容

总结构：`[Type:2][Length:4][Value:Length]`

## 文件结构

```
tlv/
├── README.md           # 说明文档
├── tlv_decoder.go      # TLV 解码器实现
├── server/
│   └── main.go         # TLV 服务器示例
└── client/
    └── main.go         # TLV 客户端示例
```

## 支持的消息类型

服务器支持以下消息类型：

- **Type 1**: Echo 消息 - 服务器会回显收到的内容，前缀为 "Echo: "
- **Type 2**: Ping 消息 - 服务器会回复 "Pong" (Type 3)
- **其他**: 未知消息类型 - 服务器会回复 "Unknown message type" (Type 999)

## 运行示例

### 启动服务器

```bash
cd example/tlv/server
go run main.go
```

服务器将监听在 `:8080` 端口。

### 运行客户端

在另一个终端窗口中：

```bash
cd example/tlv/client
go run main.go
```

客户端会依次发送：
1. Echo 消息: "Hello, TLV Server!"
2. Ping 消息: "ping"  
3. 未知类型消息: "unknown message"

## 关键特性

### TLV 解码器特性

1. **缓冲区管理**：处理不完整的数据包，自动缓存直到收到完整的 TLV 消息
2. **安全检查**：限制消息最大长度（1MB），防止恶意数据攻击
3. **大端序编码**：Type 和 Length 字段使用网络字节序（大端序）
4. **错误处理**：完善的错误处理机制

### 服务器特性

1. **事件循环**：使用 Pulse 的 `TaskTypeInEventLoop` 模式，在事件循环中处理消息
2. **异步处理**：非阻塞的消息处理
3. **多消息类型支持**：根据 Type 字段路由到不同的处理逻辑
4. **结构化日志**：使用 `slog` 提供详细的日志输出

## 扩展示例

你可以通过以下方式扩展这个示例：

1. **添加新的消息类型**：在服务器的 `OnData` 方法中添加新的 case
2. **修改 TLV 格式**：调整 Type 和 Length 字段的大小
3. **添加加密**：在 Value 字段中实现数据加密
4. **批量消息**：处理包含多个 TLV 消息的数据包

## 技术细节

### 解码流程

1. 数据到达时先追加到内部缓冲区
2. 检查是否有足够的数据读取 TLV 头部（6字节）
3. 解析 Type 和 Length 字段
4. 验证 Length 的合理性
5. 检查是否有足够的数据读取完整的 Value
6. 提取 Value 数据并创建 TLVMessage
7. 更新缓冲区，保留剩余的数据

### 性能考虑

- 使用内存池来减少内存分配
- 大端序解码使用 `binary.BigEndian` 的高效实现
- 缓冲区重用减少 GC 压力 