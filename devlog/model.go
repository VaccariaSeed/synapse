package devlog

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
	"runtime"
	"strings"
	"sync"
)

var vsLogger sync.Map

func CreateDeviceLogger(deviceName, deviceType string, deviceId int) error {
	if strings.TrimSpace(deviceName) == "" {
		return errors.New("device name is empty")
	}
	if strings.TrimSpace(deviceType) == "" {
		return errors.New("device type is empty")
	}
	lm := &LogModel{
		DeviceName: deviceName,
		DeviceId:   deviceId,
		DeviceType: deviceType,
		Key:        fmt.Sprintf("<name:%s type:%s id:%d>", deviceName, deviceType, deviceId),
	}
	sugarLogger, err := createLogConf(deviceName)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		lm.SugaredLogger = sugarLogger.Named(lm.Key)
	} else {
		lm.SugaredLogger = sugarLogger
	}
	vsLogger.Store(deviceName, lm)
	return nil
}

type LogModel struct {
	DeviceName string
	DeviceId   int
	DeviceType string
	Key        string
	*zap.SugaredLogger
}

func Load(deviceName string) *LogModel {
	if lg, ok := vsLogger.Load(deviceName); ok {
		return lg.(*LogModel)
	}
	return defaultLogger
}
