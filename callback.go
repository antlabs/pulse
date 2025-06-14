package pulse

type OnOpen func(c *Conn, err error)

type OnData func(c *Conn, data []byte)

type OnClose func(c *Conn, err error)

type Callback interface {
	OnOpen(c *Conn)
	OnData(c *Conn, data []byte)
	OnClose(c *Conn, err error)
}

type toCallback struct {
	onOpen  OnOpen
	onData  OnData
	onClose OnClose
}

func (t *toCallback) OnOpen(c *Conn) {
	t.onOpen(c, nil)
}

func (t *toCallback) OnData(c *Conn, data []byte) {
	t.onData(c, data)
}

func (t *toCallback) OnClose(c *Conn, err error) {
	t.onClose(c, err)
}

// 工具函数，回调函数转成callback接口
func ToCallback(onOpen OnOpen, onData OnData, onClose OnClose) Callback {
	return &toCallback{
		onOpen:  onOpen,
		onData:  onData,
		onClose: onClose,
	}
}
