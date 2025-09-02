package vtcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var LoginFail = errors.New("login fail, maybe username or password error")
var LoginResponseCodeError = errors.New("login response code error")

type clientHandleFunc func(topic string, data []byte) (responseTopic string, responseData []byte, err error)
type ClientHandleOpt struct {
	handles map[string]clientHandleFunc
}

func (c *ClientHandleOpt) Append(topic string, handle clientHandleFunc) {
	c.handles[topic] = handle
}

type TCPClientHandle interface {
	LoseConnect(err error) //断开连接
	Connected(localAddr string, remoteAddr string)
	LoseLogin(err error)               //登录失败
	AppendHandle(opt *ClientHandleOpt) //加载消息处理配置
	DefaultDataRequestHandler(topic string, data []byte) (responseTopic string, responseData []byte, err error)
}

type TidingsTCPClient struct {
	Ip                 string
	Port               int  //端口号
	ConnectTimeout     int  //连接超时时间
	WriteTimeout       int  //写超时
	ReadTimeout        int  //读超时
	AutoConnect        bool //是否开启自动重连
	ClientId           string
	Username           string
	Password           string
	loseConnectTimeOut int //重连间隔刷新
	ClientHandle       TCPClientHandle
	conn               net.Conn
	reader             *bufio.Reader
	isConnected        atomic.Bool
	isLogin            atomic.Bool
	frameIndexEnum     byte
	frameIndexEnumLock sync.Mutex
	HeartBeatTimer     int
	decodec            *SwiftVehicle
	clientHandleOpt    *ClientHandleOpt
	responseMap        sync.Map
	futureTimestamp    atomic.Int64
}

// 刷新重连间隔时间
func (t *TidingsTCPClient) connectLoseTimeOut() int {
	if t.loseConnectTimeOut == 0 {
		t.loseConnectTimeOut = 1
		return t.loseConnectTimeOut
	}
	t.loseConnectTimeOut = t.loseConnectTimeOut * 2
	if t.loseConnectTimeOut > 60*10 {
		t.loseConnectTimeOut = 60 * 10
	}
	return t.loseConnectTimeOut
}

func (t *TidingsTCPClient) flushConnectFlag(flag bool) {
	t.isConnected.Store(flag)
}

func (t *TidingsTCPClient) isConnectFlag() bool {
	return t.isConnected.Load()
}

func (t *TidingsTCPClient) flushLoginFlag(flag bool) {
	t.isLogin.Store(flag)
}

func (t *TidingsTCPClient) isLoginFlag() bool {
	return t.isLogin.Load()
}

func (t *TidingsTCPClient) Connect() error {
	if t.ClientHandle == nil {
		return errors.New("clientHandle is nil")
	}
	t.clientHandleOpt = &ClientHandleOpt{make(map[string]clientHandleFunc)}
	t.decodec = &SwiftVehicle{}
	var timeOut int
	go func() {
		var err error
		for {
			t.conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", t.Ip, t.Port), time.Duration(t.ConnectTimeout)*time.Second)
			if err != nil {
				if t.AutoConnect == false {
					t.ClientHandle.LoseConnect(err)
					break
				}
				timeOut = t.connectLoseTimeOut()
				t.ClientHandle.LoseConnect(err)
				time.Sleep(time.Duration(timeOut) * time.Second)
				continue
			}
			t.reader = bufio.NewReader(t.conn)
			t.flushConnectFlag(true)
			err = t.login()
			if err != nil {
				t.ClientHandle.LoseLogin(err)
				_ = t.Close()
				return
			}
			t.flushLoginFlag(true)
			t.ClientHandle.Connected(t.conn.LocalAddr().String(), t.conn.RemoteAddr().String())
			t.ClientHandle.AppendHandle(t.clientHandleOpt)
			ctx, cancel := context.WithCancel(context.Background())
			go t.keepAlive(ctx)
			//登录成功，开始处理数据
			err = t.handler()
			if err == nil {
				cancel()
				t.flushLoginFlag(false)
				t.flushConnectFlag(false)
				_ = t.conn.Close()
				return
			} else {
				t.ClientHandle.LoseConnect(err)
				cancel()
				_ = t.Close()
				continue
			}
		}
	}()
	return nil
}

func (t *TidingsTCPClient) nextFrameIndex() byte {
	t.frameIndexEnumLock.Lock()
	defer t.frameIndexEnumLock.Unlock()
	if t.frameIndexEnum == 255 {
		t.frameIndexEnum = 0
		return t.frameIndexEnum
	}
	t.frameIndexEnum = t.frameIndexEnum + 1
	return t.frameIndexEnum - 1
}

func (t *TidingsTCPClient) login() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	ticker := time.NewTicker(10 * time.Second)
	defer cancel()
	defer ticker.Stop()
	loginFrame := CreateClientFrame(t.ClientId)
	loginFrame.FrameIndex = t.nextFrameIndex()
	loginFrame.FrameType = LoginRequest
	lh := &LoginRequestMsg{
		ClientId:  t.ClientId,
		Username:  t.Username,
		Password:  t.Password,
		HeartBeat: t.HeartBeatTimer,
	}
	lh.Ts = time.Now().UnixMilli()
	data, _ := json.Marshal(lh)
	loginFrame.Data = data
	loginFrame.DataSize = uint32(len(data))
	_ = t.Send(loginFrame)
	for {
		select {
		case <-ctx.Done():
			return context.DeadlineExceeded
		case <-ticker.C:
			lh.Ts = time.Now().UnixMilli()
			data, _ = json.Marshal(lh)
			loginFrame.Data = data
			loginFrame.DataSize = uint32(len(data))
			_ = t.Send(loginFrame)
		default:
			err := t.read()
			if err != nil {
				continue
			}
			frameType := t.decodec.FrameType
			if frameType != LoginResponse || len(t.decodec.Data) == 0 {
				continue
			}
			code := t.decodec.Data[0]
			if code == 0x00 {
				//登录成功
				return nil
			} else if code == 0xff {
				//登录失败
				return LoginFail
			} else {
				//错误码
				return LoginResponseCodeError
			}
		}
	}
}

func (t *TidingsTCPClient) read() error {
	_ = t.conn.SetReadDeadline(time.Now().Add(time.Duration(t.ReadTimeout) * time.Second))
	return t.decodec.Decode(t.reader)
}

func (t *TidingsTCPClient) Send(frame *SwiftVehicle) error {
	data, err := frame.Encode()
	if err != nil {
		return err
	}
	_ = t.conn.SetWriteDeadline(time.Now().Add(time.Duration(t.WriteTimeout) * time.Second))
	_, err = t.conn.Write(data)
	return err
}

func (t *TidingsTCPClient) isConnectionClosed(err error) bool {
	// 最常见的连接关闭错误
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return false
	}
	// 系统调用错误：连接重置、拒绝、管道破裂等
	if errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	// 网络不可达错误
	if errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}
	// 检查net.OpError（网络操作错误）
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// 只关心读写操作错误
		if opErr.Op == "read" || opErr.Op == "write" {
			return true
		}
	}
	// 检查错误消息中是否包含断开连接的关键词
	errMsg := strings.ToLower(err.Error())
	disconnectKeywords := []string{
		"connection reset",
		"connection refused",
		"broken pipe",
		"forcibly closed",
		"no route to host",
		"network is unreachable",
	}
	for _, keyword := range disconnectKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}
	return false
}

func (t *TidingsTCPClient) Close() error {
	//发送断开连接信息
	logout := CreateClientFrame(t.ClientId)
	logout.FrameIndex = t.nextFrameIndex()
	logout.FrameType = LogOutRequest
	logout.TopicSize = 0
	logout.Topic = ""
	logout.DataSize = 0
	logout.Data = nil
	return t.Send(logout)
}

func (t *TidingsTCPClient) handler() error {
	var err error
	for {
		err = t.read()
		if err != nil {
			flag := t.isConnectionClosed(err)
			if flag {
				return err
			}
			continue
		}
		switch t.decodec.FrameType {
		case HeartBeatResponse:
			continue
		case LogOutResponse:
			//退出登录
			return nil
		case DataRequest:
			//请求的数据
			t.serverDataRequestHandler()
		case DataResponse:
			//返回的数据
			t.serverDataResponseHandler()
		}
	}
}

func (t *TidingsTCPClient) serverDataRequestHandler() {
	var responseMap = make([]clientHandleFunc, 0)
	for topic, handle := range t.clientHandleOpt.handles {
		if topic == t.decodec.Topic {
			responseMap = append(responseMap, handle)
		} else if strings.TrimSpace(topic) == "/#" || strings.TrimSpace(topic) == "/" || strings.TrimSpace(topic) == "#" {
			responseMap = append(responseMap, handle)
		} else {
			if strings.HasSuffix(topic, "/#") {
				if strings.Contains(t.decodec.Topic, strings.TrimSuffix(topic, "#")) {
					responseMap = append(responseMap, handle)
				}
			}
		}
	}
	var responseTopic string
	var responseMsg []byte
	var err error
	if len(responseMap) == 0 {
		responseTopic, responseMsg, err = t.ClientHandle.DefaultDataRequestHandler(t.decodec.Topic, t.decodec.Data)
		_ = t.dataResponseHandler(responseTopic, responseMsg, err)
	} else {
		for _, dataRequestHandle := range responseMap {
			responseTopic, responseMsg, err = dataRequestHandle(t.decodec.Topic, t.decodec.Data)
			_ = t.dataResponseHandler(responseTopic, responseMsg, err)
		}
	}
}

func (t *TidingsTCPClient) dataResponseHandler(topic string, msg []byte, err error) error {
	if topic == "" || len(topic) == 0 {
		topic = t.decodec.Topic
	}
	frameIndex := t.decodec.FrameIndex
	response := CreateClientFrame(t.ClientId)
	response.FrameIndex = frameIndex
	response.TopicSize = uint8(len(topic))
	response.Topic = topic
	if err != nil {
		response.DataSize = uint32(len(err.Error()) + 1)
		response.Data = append([]byte{1}, []byte(err.Error())...)
	} else {
		response.DataSize = uint32(len(msg) + 1)
		response.Data = append([]byte{0}, msg...)
	}
	response.FrameType = DataResponse
	return t.send(response)
}

func (t *TidingsTCPClient) send(data *SwiftVehicle) error {
	t.futureTimestamp.Store(time.Now().Add(time.Duration(t.HeartBeatTimer) * time.Second).Unix())
	dataArr, err := data.Encode()
	if err != nil {
		return err
	}
	err = t.conn.SetWriteDeadline(time.Now().Add(time.Duration(t.WriteTimeout) * time.Second))
	if err != nil {
		return err
	}
	_, err = t.conn.Write(dataArr)
	return err
}

func (t *TidingsTCPClient) serverDataResponseHandler() {
	frameIndex := t.decodec.FrameIndex
	response := t.decodec.Data
	if channel, ok := t.responseMap.Load(frameIndex); ok {
		channel.(chan []byte) <- response
		close(channel.(chan []byte))
		t.responseMap.Delete(frameIndex)
	}
}

// 心跳
func (t *TidingsTCPClient) keepAlive(ctx context.Context) {
	stopTimer := t.HeartBeatTimer / 3
	if stopTimer <= 1 {
		stopTimer = 1
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if time.Now().Unix() < t.futureTimestamp.Load() {
				time.Sleep(time.Duration(stopTimer) * time.Second)
				continue
			}
			keep := CreateClientFrame(t.ClientId)
			keep.FrameType = t.nextFrameIndex()
			keep.FrameType = HeartBeatRequest
			keep.Topic = ""
			keep.Data = nil
			_ = t.send(keep)
		}
	}
}

func (t *TidingsTCPClient) SendMsg(topic string, data string) error {
	frameIndex := t.nextFrameIndex()
	return t.sendMsgByFrameIndex(topic, data, frameIndex)
}

func (t *TidingsTCPClient) sendMsgByFrameIndex(topic string, data string, frameIndex byte) error {
	if !t.isConnected.Load() || !t.isLogin.Load() {
		return errors.New("not connected")
	}
	if topic == "" || len(topic) == 0 {
		return errors.New("invalid topic")
	}
	msg := CreateClientFrame(t.ClientId)
	msg.FrameType = DataRequest
	msg.FrameIndex = frameIndex
	msg.Topic = topic
	msg.TopicSize = uint8(len(data))
	if data == "" {
		msg.Data = nil
		msg.DataSize = 0
	} else {
		msg.Data = []byte(data)
		msg.DataSize = uint32(len(data))
	}
	return t.Send(msg)
}

func (t *TidingsTCPClient) SendOnIdempotent(topic string, data string, idempotent int) ([]byte, error) {
	if topic == "" || len(data) == 0 {
		return nil, errors.New("topic or data is empty")
	}
	if idempotent <= 0 {
		return nil, t.SendMsg(topic, data)
	}
	frameIndex := t.nextFrameIndex()
	dataChan := make(chan []byte)
	t.responseMap.Store(frameIndex, dataChan)
	err := t.sendMsgByFrameIndex(topic, data, frameIndex)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(idempotent)*time.Second)
	defer cancel()
	for {
		select {
		case response := <-dataChan:
			flag := response[0]
			if flag == 1 {
				return nil, errors.New(string(response[1:]))
			}
			return response[1:], nil
		case <-ctx.Done():
			t.responseMap.Delete(frameIndex)
			return nil, os.ErrDeadlineExceeded
		}
	}
}
