package vtcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ServerMsgHandleFunc func(clientId string, topic string, msg []byte) (responseTopic string, responseMsg []byte, err error)

type STCPServerHandle interface {
	ClientLoginTimeOut(clientIpPort string)                                                                            //客户端登陆超时
	ClientConnected(clientAddress, clientId string)                                                                    //客户端连接成功
	ClientApplyDisConnected(clientId string)                                                                           //客户端申请断开连接
	ClientAbnormalDisConnected(clientId string, err error)                                                             //客户端异常断开连接
	ClientLogin(clientId string, username string, password string, loginTs int64) bool                                 //客户端登录请求，return true则为认定通过
	DefaultMsgHandler(clientId string, topic string, msg []byte) (responseTopic string, responseMsg []byte, err error) //收到客户端的消息, err会回复错误信息
	AppendMessageHandleFunc(msgHandle *ServerMsgHandle)                                                                //添加客户端消息处理方法
}

type ConnectOpt struct {
	Port           int //端口号
	ConnectTimeout int //连接超时时间
	WriteTimeout   int //写超时
	ReadTimeout    int //读超时
}

type MsgTopicHandle struct {
	clientId string
	topic    string
	handle   ServerMsgHandleFunc
}

type ServerMsgHandle struct {
	handles []*MsgTopicHandle
}

func (s *ServerMsgHandle) Append(clientId, topic string, handle ServerMsgHandleFunc) {
	hl := &MsgTopicHandle{
		clientId: clientId,
		topic:    topic,
		handle:   handle,
	}
	s.handles = append(s.handles, hl)
}

func (h *ServerMsgHandle) arrange(clientId string) map[string]ServerMsgHandleFunc {
	result := make(map[string]ServerMsgHandleFunc)
	for _, handle := range h.handles {
		if handle.clientId == clientId || strings.TrimSpace(handle.clientId) == "" || strings.TrimSpace(handle.clientId) == "*" {
			result[handle.topic] = handle.handle
		}
	}
	return result
}

type TidingsTCPServer struct {
	config          *ConnectOpt //连接参数
	serverHandle    STCPServerHandle
	listener        net.Listener
	clientLock      sync.Mutex
	clientSnapshot  map[string]*clientSide
	serverMsgHandle *ServerMsgHandle
	closed          atomic.Bool
}

func NewSwiftTCPServer(config *ConnectOpt, handle STCPServerHandle) (*TidingsTCPServer, error) {
	if handle == nil {
		return nil, errors.New("handle must not be nil")
	}
	if config == nil {
		return nil, errors.New("config must not be nil")
	}
	return &TidingsTCPServer{
		config:          config,
		serverHandle:    handle,
		clientSnapshot:  make(map[string]*clientSide),
		serverMsgHandle: &ServerMsgHandle{handles: make([]*MsgTopicHandle, 0)},
	}, nil
}

func (t *TidingsTCPServer) Close() error {
	t.closed.Store(true)
	t.clientLock.Lock()
	defer t.clientLock.Unlock()
	for _, client := range t.clientSnapshot {
		_ = client.Close()
	}
	return t.listener.Close()
}

func (t *TidingsTCPServer) Open() error {
	if t.serverHandle == nil {
		return errors.New("handle must not be nil")
	}
	if t.config == nil {
		return errors.New("config must not be nil")
	}
	t.closed.Store(false)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", t.config.Port))
	if err != nil {
		return err
	}
	t.listener = listener
	t.serverHandle.AppendMessageHandleFunc(t.serverMsgHandle)
	go func() {
		for {
			conn, err := t.listener.Accept()
			if err != nil {
				continue
			}
			go t.clientHandle(conn)
		}
	}()
	return nil
}

func (t *TidingsTCPServer) clientHandle(conn net.Conn) {
	ipPort := conn.RemoteAddr().String()
	client := &clientSide{ipPort: ipPort, conn: conn, reader: bufio.NewReader(conn), decodec: &SwiftVehicle{}, defaultMsgHandler: t.serverHandle.DefaultMsgHandler, writeTimeout: t.config.WriteTimeout, readTimeout: t.config.ReadTimeout}
	//验证登录
	clientId, username, password, frameIndex, ts, err := client.checkLogin()
	if err != nil {
		_ = client.conn.Close()
		if errors.Is(err, context.DeadlineExceeded) {
			t.serverHandle.ClientLoginTimeOut(ipPort)
		} else {
			t.serverHandle.ClientAbnormalDisConnected(ipPort, err)
		}
		return
	}
	loginFlag := t.serverHandle.ClientLogin(clientId, username, password, ts)
	//登录
	if !loginFlag {
		client.loginResponse(frameIndex, 0xFF)
		_ = conn.Close()
		return
	}
	client.handles = t.serverMsgHandle.arrange(clientId)
	//发送登录成功信息
	client.loginResponse(frameIndex, 0x00)
	//登录成功
	t.clientLock.Lock()
	t.clientSnapshot[clientId] = client
	t.clientLock.Unlock()
	t.serverHandle.ClientConnected(ipPort, clientId)
	//isActive是不是客户端主动断开的连接,isLogOut是不是client主动发送的断开报文
	isLogOut, decodeErr := client.msgProcessor()
	if !t.closed.Load() {
		if isLogOut {
			t.serverHandle.ClientApplyDisConnected(clientId)
		} else {
			t.serverHandle.ClientAbnormalDisConnected(clientId, decodeErr)
		}
	}
	_ = client.Close()
	t.clientLock.Lock()
	delete(t.clientSnapshot, clientId)
	t.clientLock.Unlock()
}

func (t *TidingsTCPServer) Send(clientId string, topic string, data string) error {
	var client *clientSide
	t.clientLock.Lock()
	if cli, ok := t.clientSnapshot[clientId]; ok {
		client = cli
		t.clientLock.Unlock()
	} else {
		t.clientLock.Unlock()
		return errors.New("client not exist")
	}
	frameIndex := client.nextFrameIndex()
	return client.sendMsg(frameIndex, topic, data, DataRequest)
}

func (t *TidingsTCPServer) SendOnIdempotent(clientId string, topic string, data string, idempotent int) ([]byte, error) {
	if topic == "" || len(data) == 0 {
		return nil, errors.New("topic or data is empty")
	}
	if idempotent <= 0 {
		return nil, t.Send(clientId, topic, data)
	}
	var client *clientSide
	t.clientLock.Lock()
	if cli, ok := t.clientSnapshot[clientId]; ok {
		client = cli
		t.clientLock.Unlock()
	} else {
		t.clientLock.Unlock()
		return nil, errors.New("client not exist")
	}
	frameIndex := client.nextFrameIndex()
	dataChan := make(chan []byte)
	client.responseMap.Store(frameIndex, dataChan)
	err := client.sendMsg(frameIndex, topic, data, DataRequest)
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
			client.responseMap.Delete(frameIndex)
			return nil, os.ErrDeadlineExceeded
		}
	}

}

func (t *TidingsTCPServer) CloseClient(clientId string) {
	if client, ok := t.clientSnapshot[clientId]; ok {
		_ = client.Close()
	}
	t.clientLock.Lock()
	delete(t.clientSnapshot, clientId)
	t.clientLock.Unlock()
}
