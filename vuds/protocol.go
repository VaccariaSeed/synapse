package vuds

type ClientType string

const (
	MASTER ClientType = "master"
	SLAVE  ClientType = "slave"
)

type UnexpectedClose func(err error) //意外关闭
type SlaveMessageReceive func(msg []byte) []byte

type UDSMessage struct {
	Source ClientType `json:"source"`
	Index  int64      `json:"index"`
	Data   []byte     `json:"data"`
}
