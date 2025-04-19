package pulse

type OnOpen func(c *Conn, err error)

type OnData[T any] func(c *Conn, data T)

type OnClose func(c *Conn, err error)

type Callback[T any] interface {
	OnOpen(c *Conn, err error)
	// T 是支持[]byte类型转换的
	OnData(c *Conn, data T)
	OnClose(c *Conn, err error)
}

type Decoder[T any] interface {
	Decode(data []byte) (T, []byte, error)
}

type toCallback[T any] struct {
	onOpen  OnOpen
	onData  OnData[T]
	onClose OnClose
}

func (t *toCallback[T]) OnOpen(c *Conn, err error) {
	t.onOpen(c, err)
}

func (t *toCallback[T]) OnData(c *Conn, data T) {
	t.onData(c, data)
}

func (t *toCallback[T]) OnClose(c *Conn, err error) {
	t.onClose(c, err)
}

// 工具函数，回调函数转成callback接口
func ToCallback[T any](onOpen OnOpen, onData OnData[T], onClose OnClose) Callback[T] {
	return &toCallback[T]{
		onOpen:  onOpen,
		onData:  onData,
		onClose: onClose,
	}
}
