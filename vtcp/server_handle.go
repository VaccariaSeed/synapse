package vtcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type defaultMsgHandlerFunc func(clientId string, topic string, msg []byte) (responseTopic string, responseMsg []byte, err error)

type clientSide struct {
	ipPort                string
	clientId              string
	username              string
	password              string
	heartbeat             int64
	conn                  net.Conn
	reader                *bufio.Reader
	decodec               *SwiftVehicle //解析器
	handles               map[string]ServerMsgHandleFunc
	loginTs               int64
	lastCommunicationTime atomic.Int64 //最后一次通讯时间
	lastCommunicationLock sync.Mutex
	cancel                context.CancelFunc
	activeLogOut          bool
	defaultMsgHandler     defaultMsgHandlerFunc
	frameIndexEnum        byte
	frameIndexLock        sync.Mutex
	lastFrameIndex        byte
	readTimeout           int
	writeTimeout          int
	responseMap           sync.Map
}

func (s *clientSide) nextFrameIndex() byte {
	s.frameIndexLock.Lock()
	defer s.frameIndexLock.Unlock()
	if s.frameIndexEnum == 255 {
		s.frameIndexEnum = 0
		return s.frameIndexEnum
	}
	s.frameIndexEnum = s.frameIndexEnum + 1
	return s.frameIndexEnum - 1
}

func (s *clientSide) Close() error {
	s.lastCommunicationLock.Lock()
	if s.cancel != nil {
		s.activeLogOut = true
		s.cancel()
		s.cancel = nil
	}
	s.lastCommunicationLock.Unlock()
	return s.conn.Close()
}

func (s *clientSide) checkLogin() (clientId, username, password string, frameIndex byte, ts int64, err error) {
	timeout := time.After(30 * time.Second) // 30秒超时
	for {
		select {
		case <-timeout:
			return "", "", "", 0, 0, context.DeadlineExceeded
		default:
			//从这里解析登录帧
			err = s.read()
			if err != nil {
				continue
			}
			if s.decodec.FrameType == LoginRequest {
				//解析登录帧
				var login = &LoginRequestMsg{}
				err = json.Unmarshal(s.decodec.Data, login)
				if err != nil {
					continue
				}
				s.clientId = login.ClientId
				s.username = login.Username
				s.password = login.Password
				s.heartbeat = int64(login.HeartBeat)
				s.loginTs = int64(login.Ts)
				s.flushLastCommunicationTime()
				return s.clientId, s.username, s.password, s.decodec.FrameIndex, s.loginTs, nil
			}

		}
	}
}

func (s *clientSide) read() error {
	_ = s.conn.SetReadDeadline(time.Now().Add(time.Duration(s.readTimeout) * time.Second))
	//通过s.decodec解析
	return s.decodec.Decode(s.reader)
}

func (s *clientSide) flushLastCommunicationTime() {
	s.lastCommunicationTime.Store(time.Now().Unix())
}

func (s *clientSide) selectLastCommunicationTime() int64 {
	return s.lastCommunicationTime.Load()
}

func (s *clientSide) loginResponse(frameIndex byte, code int) {
	//todo 发送登录成功确认帧
	sv, _ := CreateServerFrame(frameIndex, "")
	sv.Data = []byte{byte(code)}
	sv.DataSize = 1
	sv.FrameType = LoginResponse
	_ = s.send(sv)
}

func (s *clientSide) checkLastTime(ctx context.Context) {
	var now int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
			now = time.Now().Unix()
			if (now - s.selectLastCommunicationTime()) >= s.heartbeat*3 {
				s.lastCommunicationLock.Lock()
				if s.cancel != nil {
					s.cancel()
					s.cancel = nil
				}

				s.lastCommunicationLock.Unlock()
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// isActive是不是客户端主动断开的连接,isLogOut是不是client主动发送的断开报文
func (s *clientSide) msgProcessor() (isLogOut bool, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.checkLastTime(ctx)
	for {
		select {
		case <-ctx.Done():
			return s.activeLogOut, errors.New("server close client connection")
		default:
			err = s.read()
			if err != nil {
				if err == io.EOF {
					return false, err
				}
				continue
			}
			//根据类型进行分类
			switch s.decodec.FrameType {
			case HeartBeatRequest:
				s.heartBeatHandle()
			case LogOutRequest:
				s.logOutHandle()
				return true, nil
			case DataRequest:
				s.DataRequestHandle()
			case DataResponse:
				s.DataResponseHandle()
			default:
				continue
			}
		}
	}
}

func (s *clientSide) heartBeatHandle() {
	s.flushLastCommunicationTime()
	//返回数据
	frameIndex := s.decodec.FrameIndex
	hbResponse, _ := CreateServerFrame(frameIndex, "")
	hbResponse.FrameType = HeartBeatResponse
	hbResponse.FrameIndex = frameIndex
	hbResponse.DataSize = 0x00
	_ = s.send(hbResponse)
}

func (s *clientSide) logOutHandle() {
	frameIndex := s.decodec.FrameIndex
	logout, _ := CreateServerFrame(frameIndex, "")
	logout.FrameType = LogOutResponse
	_ = s.send(logout)
	s.activeLogOut = true
	s.cancel()
}

func (s *clientSide) DataRequestHandle() {
	s.flushLastCommunicationTime()
	frameIndex := s.decodec.FrameIndex
	if frameIndex == s.lastFrameIndex {
		return
	}
	s.lastFrameIndex = frameIndex
	requestData := s.decodec.Data
	requestTopic := s.decodec.Topic
	//组建去路
	var responseMap []ServerMsgHandleFunc
	for topic, handle := range s.handles {
		if topic == requestTopic {
			responseMap = append(responseMap, handle)
		} else if strings.TrimSpace(topic) == "/#" || strings.TrimSpace(topic) == "/" || strings.TrimSpace(topic) == "#" {
			responseMap = append(responseMap, handle)
		} else {
			if strings.HasSuffix(topic, "/#") {
				if strings.Contains(requestTopic, strings.TrimSuffix(topic, "#")) {
					responseMap = append(responseMap, handle)
				}
			}
		}
	}
	var responseTopic string
	var responseMsg []byte
	var err error
	if len(responseMap) == 0 {
		responseTopic, responseMsg, err = s.defaultMsgHandler(s.clientId, requestTopic, requestData)
		_ = s.dataResponseHandler(responseTopic, responseMsg, err)
	} else {
		for _, handle := range responseMap {
			responseTopic, responseMsg, err = handle(s.clientId, requestTopic, requestData)
			_ = s.dataResponseHandler(responseTopic, responseMsg, err)
		}
	}
}

func (s *clientSide) dataResponseHandler(topic string, msg []byte, err error) error {
	if topic == "" || len(topic) == 0 {
		topic = s.decodec.Topic
	}
	frameIndex := s.decodec.FrameIndex
	response, _ := CreateServerFrame(frameIndex, "")
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
	return s.send(response)
}

func (s *clientSide) DataResponseHandle() {
	s.flushLastCommunicationTime()
	frameIndex := s.decodec.FrameIndex
	response := s.decodec.Data
	if channel, ok := s.responseMap.Load(frameIndex); ok {
		channel.(chan []byte) <- response
		close(channel.(chan []byte))
		s.responseMap.Delete(frameIndex)
	}
}

func (s *clientSide) send(data *SwiftVehicle) error {
	dataArr, err := data.Encode()
	if err != nil {
		return err
	}
	err = s.conn.SetWriteDeadline(time.Now().Add(time.Duration(s.writeTimeout) * time.Second))
	if err != nil {
		return err
	}
	_, err = s.conn.Write(dataArr)
	return err
}

func (s *clientSide) sendMsg(frameIndex byte, topic string, data string, frameType byte) error {
	if topic == "" || len(topic) == 0 {
		return errors.New("invalid topic")
	}
	frame, _ := CreateServerFrame(frameIndex, data)
	frame.FrameIndex = frameIndex
	frame.FrameType = frameType
	frame.Topic = topic
	frame.TopicSize = uint8(len(topic))
	return s.send(frame)
}
