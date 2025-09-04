package header

import "bufio"

// ProtocolParser 规约接口
type ProtocolParser interface {
	Encode() ([]byte, error)                  //编码
	EncodeToString() (string, error)          //编码成字符串
	Decode(msg []byte) error                  //解码
	DecodeByReader(reader bufio.Reader) error //通过reader解码
	Name() string                             //返回规约类型
}
