package pulse

type Callback interface {
	OnOpen(c *Conn, err error)
	OnData(c *Conn, data []byte)
	OnClose(c *Conn, err error)
}
