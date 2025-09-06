package protocol

import "bufio"

// PacketParser 规约结构接口
type PacketParser interface {
	Encode() ([]byte, error)                   //编码
	EncodeToString() (string, error)           //编码成字符串
	Decode(msg []byte) error                   //解码
	DecodeByReader(reader *bufio.Reader) error //通过reader解码
	Name() string                              //返回规约类型
	Cs([]byte) []byte                          //生成校验
}
