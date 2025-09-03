package vuds

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
	"syscall"
	"time"
)

type UDSClient struct {
	Sock                       string
	ClientType                 ClientType
	msgClientType              ClientType
	socketPath                 string
	RetrySize                  int                 //重试次数
	RetryDelay                 int                 //重试间隔，单位秒
	ReadTimeout                int                 //读超时，单位秒
	UnexpectedCloseHandler     UnexpectedClose     //意外关闭回调
	SlaveMessageReceiveHandler SlaveMessageReceive //收到slave消息的回调
	isConnected                bool                //是否已经建立连接
	decodec                    *UDSMessage
	responseMap                sync.Map
	lock                       sync.Mutex
	connReady                  bool
	conn                       net.Conn
	reader                     *bufio.Reader
	cancel                     context.CancelFunc
}

func (u *UDSClient) Close() error {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.isConnected = false
	u.connReady = false
	if u.cancel != nil {
		u.cancel()
		u.cancel = nil
	}
	return u.conn.Close()
}

func (u *UDSClient) IsConnected() bool {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.isConnected
}

func (u *UDSClient) isConnReady() bool {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.connReady
}

func (u *UDSClient) Connect() error {
	if strings.TrimSpace(u.Sock) == "" {
		return errors.New("sock is empty")
	}
	if u.RetrySize <= 0 {
		u.RetrySize = 3
	}
	if u.RetryDelay <= 0 {
		u.RetryDelay = 5
	}
	if u.ReadTimeout <= 0 {
		u.ReadTimeout = 5
	}
	if u.decodec == nil {
		u.decodec = &UDSMessage{}
	}
	if u.ClientType != MASTER && u.ClientType != SLAVE {
		return errors.New("unsupport client type")
	}
	u.socketPath = fmt.Sprintf("/tmp/%s.sock", strings.TrimSpace(u.Sock))
	switch u.ClientType {
	case MASTER:
		return u.masterConnect()
	default:
		//slave
		return u.slaveConnect()
	}
}

func (u *UDSClient) slaveConnect() error {
	u.msgClientType = MASTER
	for i := 0; i < u.RetrySize; i++ {
		if _, err := os.Stat(u.socketPath); err == nil {
			break
		}
		time.Sleep(time.Duration(u.RetryDelay) * time.Second)
		if i == u.RetrySize-1 {
			return errors.New("not found socket")
		}
	}
	var err error
	u.conn, err = net.Dial("unix", u.socketPath)
	if err != nil {
		return err
	}
	u.reader = bufio.NewReader(u.conn)
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	u.lock.Lock()
	u.isConnected = true
	u.connReady = true
	u.lock.Unlock()
	go func() {
		err = u.listenIn(ctx)
		_ = u.Close()
		if err != nil {
			if u.UnexpectedCloseHandler != nil {
				u.UnexpectedCloseHandler(err)
			}
		}
	}()
	return nil
}

func (u *UDSClient) masterConnect() error {
	u.msgClientType = SLAVE
	_ = os.Remove(u.socketPath)
	listener, err := net.Listen("unix", u.socketPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	u.lock.Lock()
	u.isConnected = true
	u.lock.Unlock()
	go func() {
		for {
			u.conn, err = listener.Accept()
			if err != nil {
				continue
			}
			u.lock.Lock()
			u.connReady = true
			u.lock.Unlock()
			break
		}
		u.reader = bufio.NewReader(u.conn)
		go func() {
			err = u.listenIn(ctx)
			_ = u.Close()
			if err != nil {
				if u.UnexpectedCloseHandler != nil {
					u.UnexpectedCloseHandler(err)
				}
			}
		}()
	}()
	return nil
}

func (u *UDSClient) listenIn(ctx context.Context) error {
	var line string
	var resp []byte
	var err error
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_ = u.conn.SetDeadline(time.Now().Add(time.Duration(u.ReadTimeout) * time.Second))
			line, err = u.reader.ReadString('\n')
			if err != nil {
				//io.EOF 对方程序退出
				//net.ErrClosed 连接被关闭
				//errors.Is(err, syscall.EPIPE) 管道破裂，对方可能已退出
				//errors.Is(err, syscall.EBADF) 错误的文件描述符
				//os.IsNotExist(err) 文件不存在
				//os.IsPermission(err) 没有权限访问 socket 文件
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.EBADF) || os.IsNotExist(err) || os.IsPermission(err) {
					return err
				}
				continue
			}
			err = json.Unmarshal([]byte(line), &u.decodec)
			if err != nil || u.decodec.Source != u.msgClientType {
				continue
			}
			if ch, ok := u.responseMap.Load(u.decodec.Index); ok {
				u.responseMap.Delete(u.decodec.Index)
				ch.(chan []byte) <- u.decodec.Data
				close(ch.(chan []byte))
			} else if u.SlaveMessageReceiveHandler != nil {
				resp = u.SlaveMessageReceiveHandler(u.decodec.Data)
				if resp != nil || len(resp) != 0 {
					_ = u.Send(resp)
				}
			}
		}
	}
}

func (u *UDSClient) Send(req []byte) error {
	if !u.IsConnected() || !u.isConnReady() {
		return errors.New("already connected")
	}
	if req == nil || len(req) == 0 {
		return errors.New("req is empty")
	}
	msg := UDSMessage{
		Source: u.ClientType,
		Index:  time.Now().UnixMilli(),
		Data:   req,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = u.conn.Write(append(data, '\n'))
	return err
}

func (u *UDSClient) SendByWriteTimeOut(req []byte, timeout int) error {
	if timeout <= 0 {
		return errors.New("timeout must be > 0")
	}
	err := u.conn.SetWriteDeadline(time.Now().Add(time.Duration(timeout) * time.Second))
	if err != nil {
		return err
	}
	return u.Send(req)
}

// SendAndWaitForReply 发送并等待回复
func (u *UDSClient) SendAndWaitForReply(req []byte, timeout int) ([]byte, error) {
	if timeout <= 0 {
		return nil, errors.New("timeout must be > 0")
	}
	if !u.IsConnected() || !u.isConnReady() {
		return nil, errors.New("already connected")
	}
	if req == nil || len(req) == 0 {
		return nil, errors.New("req is empty")
	}
	index := time.Now().UnixMilli()
	msg := UDSMessage{
		Source: u.ClientType,
		Index:  index,
		Data:   req,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	_, err = u.conn.Write(append(data, '\n'))
	if err != nil {
		return nil, err
	}
	ch := make(chan []byte)
	u.responseMap.Store(index, ch)
	defer func() {
		if _, ok := u.responseMap.Load(index); ok {
			u.responseMap.Delete(index)
			close(ch)
		}
	}()
	//设定一个超时
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(time.Duration(timeout) * time.Second):
		return nil, errors.New("timeout")
	}
}
