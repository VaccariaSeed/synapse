package header

// Linker 通讯连接器
type Linker interface {
	Start() error                   //启动
	Close() error                   //关闭
	Name() string                   //规约连接器类型
	Send([]byte) (int, error)       //发送
	SendString(string) (int, error) //发送字符串
	Flush() error                   //刷新
}
