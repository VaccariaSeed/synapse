package vtcp

import (
	"bufio"
	"encoding/binary"
	"errors"
)

const (
	LoginRequest      = 0x00
	LoginResponse     = 0x01
	HeartBeatRequest  = 0x02
	HeartBeatResponse = 0x03
	LogOutRequest     = 0x04
	LogOutResponse    = 0x05
	DataRequest       = 0x06
	DataResponse      = 0x07
)

const (
	StartChar  byte   = 0x68
	EndChar    byte   = 0x16
	ServerAddr string = "AAAAAAAA"
)

var TitleChars = [4]byte{'B', 'Z', 'K', 'A'}

type LoginRequestMsg struct {
	ClientId  string `json:"cl"`
	Username  string `json:"un"`
	Password  string `json:"pw"`
	HeartBeat int    `json:"he"` //心跳周期
	Ts        int64  `json:"ts"` //当前时间戳
}

type SwiftVehicle struct {
	StartChar    byte   //开始字符
	TitleChars   []byte //头字符
	FrameIndex   byte   //报文id
	Control      byte   //控制域 bit0:0-服务端，1-客户端
	SendAddrLen  uint16 //逻辑地址长度
	SenderAddr   []byte //发送方逻辑地址
	FramingFlag  byte   //分帧标志，0-完整帧，1-分帧，2-结束帧
	FramingIndex uint16 //分帧序号，当为完整帧和结束帧时，该值为0
	hcs          byte   //头校验位
	SegmentChar  byte   //分割字符
	FrameType    byte   //报文类型
	TopicSize    uint8  //topic长度
	Topic        string //topic
	DataSize     uint32 //报文链路数据长度
	Data         []byte //报文链路数据
	fcs          byte   //帧校验位
	EndChar      byte   //结束字符
}

func (s *SwiftVehicle) Decode(buf *bufio.Reader) error {
	var err error
	//既如此有效头字符
	err = binary.Read(buf, binary.LittleEndian, &s.StartChar)
	if err != nil {
		return err
	}
	//如果等于0x68,则为有效
	if s.StartChar != StartChar {
		return errors.New("invalid start char")
	}
	//校验TitleChars
	s.TitleChars, err = buf.Peek(4)
	if err != nil {
		return err
	}
	var csArray []byte
	if len(s.TitleChars) != 4 || s.TitleChars[0] != TitleChars[0] || s.TitleChars[1] != TitleChars[1] || s.TitleChars[2] != TitleChars[2] || s.TitleChars[3] != TitleChars[3] {
		return errors.New("invalid title char")
	}
	csArray = append(csArray, s.TitleChars...)
	discarded, err := buf.Discard(4)
	if err != nil || discarded != 4 {
		return err
	}
	//解析报文id
	err = binary.Read(buf, binary.LittleEndian, &s.FrameIndex)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.FrameIndex)
	//解析控制域
	err = binary.Read(buf, binary.LittleEndian, &s.Control)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.Control)
	//解析逻辑地址长度
	err = binary.Read(buf, binary.BigEndian, &s.SendAddrLen)
	if err != nil {
		return err
	}
	if s.SendAddrLen <= 0 {
		return errors.New("invalid sender addr length")
	}
	csArray = append(csArray, byte(s.SendAddrLen>>8), byte(s.SendAddrLen))
	//解析发送方逻辑地址
	s.SenderAddr = make([]byte, s.SendAddrLen)
	err = binary.Read(buf, binary.LittleEndian, &s.SenderAddr)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.SenderAddr...)
	//解析分帧标志
	err = binary.Read(buf, binary.LittleEndian, &s.FramingFlag)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.FramingFlag)
	//分帧序号，当为完整帧和结束帧时，该值为0
	err = binary.Read(buf, binary.BigEndian, &s.FramingIndex)
	if err != nil {
		return err
	}
	fi := make([]byte, 2)
	binary.BigEndian.PutUint16(fi, s.FramingIndex)
	csArray = append(csArray, fi...)
	//解析hcs
	err = binary.Read(buf, binary.LittleEndian, &s.hcs)
	if err != nil {
		return err
	}
	if s.hcs != s.cs(csArray) {
		return errors.New("invalid framing fcs")
	}
	csArray = append(csArray, s.hcs)
	//解析分割字符
	err = binary.Read(buf, binary.LittleEndian, &s.SegmentChar)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.SegmentChar)
	if s.SegmentChar != StartChar {
		return errors.New("invalid segment char")
	}
	//解析报文类型
	err = binary.Read(buf, binary.LittleEndian, &s.FrameType)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.FrameType)
	//存在topic
	err = binary.Read(buf, binary.BigEndian, &s.TopicSize)
	if err != nil {
		return err
	}
	csArray = append(csArray, s.TopicSize)
	if s.TopicSize > 0 {
		var topicArray = make([]byte, s.TopicSize)
		err = binary.Read(buf, binary.LittleEndian, &topicArray)
		if err != nil {
			return err
		}
		csArray = append(csArray, topicArray...)
		s.Topic = string(topicArray)
	}
	//解析链路数据长度
	err = binary.Read(buf, binary.BigEndian, &s.DataSize)
	if err != nil {
		return err
	}
	ud := make([]byte, 4)
	binary.BigEndian.PutUint32(ud, s.DataSize)
	csArray = append(csArray, ud...)
	if s.DataSize > 0 {
		//解析链路数据
		s.Data = make([]byte, s.DataSize)
		err = binary.Read(buf, binary.LittleEndian, &s.Data)
		if err != nil {
			return err
		}
		csArray = append(csArray, s.Data...)
	}
	//解析帧校验位
	err = binary.Read(buf, binary.LittleEndian, &s.fcs)
	if err != nil {
		return err
	}
	if s.fcs != s.cs(csArray) {
		return errors.New("invalid framing fcs")
	}
	//解析结束字符
	err = binary.Read(buf, binary.LittleEndian, &s.EndChar)
	if err != nil {
		return err
	}
	if s.EndChar != EndChar {
		return errors.New("invalid end char")
	}
	return nil
}

func (s *SwiftVehicle) Encode() ([]byte, error) {
	if len(s.Topic) > 255 {
		return nil, errors.New("topic too long")
	}
	//开始字符
	encodeArray := []byte{StartChar}
	//头字符
	encodeArray = append(encodeArray, s.TitleChars...)
	//报文id
	encodeArray = append(encodeArray, s.FrameIndex)
	//控制域
	encodeArray = append(encodeArray, s.Control)
	//逻辑地址长度
	encodeArray = append(encodeArray, byte(s.SendAddrLen>>8), byte(s.SendAddrLen))
	//发送方逻辑地址
	encodeArray = append(encodeArray, s.SenderAddr...)
	//分帧标志
	encodeArray = append(encodeArray, s.FramingFlag)
	//分帧序号
	fi := make([]byte, 2)
	binary.BigEndian.PutUint16(fi, s.FramingIndex)
	encodeArray = append(encodeArray, fi...)
	//头校验位
	encodeArray = append(encodeArray, s.cs(encodeArray[1:]))
	//分割字符
	encodeArray = append(encodeArray, s.SegmentChar)
	//报文类型
	encodeArray = append(encodeArray, s.FrameType)
	if s.Topic == "" || len(s.Topic) == 0 {
		encodeArray = append(encodeArray, 0)
	} else {
		encodeArray = append(encodeArray, byte(len(s.Topic)))
		topicArray := []byte(s.Topic)
		encodeArray = append(encodeArray, topicArray...)
	}
	//链路用户数据长度
	if s.Data == nil || len(s.Data) == 0 {
		encodeArray = append(encodeArray, 0, // 取最高 8 位
			0,
			0,
			0)
	} else {
		dataLen := len(s.Data)
		encodeArray = append(encodeArray, byte(dataLen>>24), // 取最高 8 位
			byte(dataLen>>16),
			byte(dataLen>>8),
			byte(dataLen))
		//链路用户数据
		encodeArray = append(encodeArray, s.Data...)
	}
	//帧校验位
	encodeArray = append(encodeArray, s.cs(encodeArray[1:]))
	//结束字符
	return append(encodeArray, s.EndChar), nil
}

func (s *SwiftVehicle) cs(frame []byte) byte {
	var sum uint32 // 使用 uint32 来避免溢出
	for _, b := range frame {
		sum += uint32(b)
	}
	return byte(sum % 256)
}

func CreateClientFrame(clientId string) *SwiftVehicle {
	return &SwiftVehicle{
		StartChar:   StartChar,
		TitleChars:  []byte{'B', 'Z', 'K', 'A'},
		SendAddrLen: uint16(len(clientId)),
		SenderAddr:  []byte(clientId),
		SegmentChar: StartChar,
		EndChar:     EndChar,
	}
}

func CreateServerFrame(frameIndex byte, data string) (*SwiftVehicle, error) {
	sv := &SwiftVehicle{
		StartChar:   StartChar,
		TitleChars:  []byte{'B', 'Z', 'K', 'A'},
		FrameIndex:  frameIndex,
		TopicSize:   0,
		SendAddrLen: uint16(len(ServerAddr)),
		SenderAddr:  []byte(ServerAddr),
		SegmentChar: StartChar,
		EndChar:     EndChar,
	}
	if data == "" {
		sv.DataSize = 0
	} else {
		sv.DataSize = uint32(len(data))
		sv.Data = []byte(data)
	}
	return sv, nil
}
