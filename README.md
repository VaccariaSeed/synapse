# synapse
IoT components

## devlog
分设备日志，根据不同设备进行不同日志记录
#### 使用流程
```go
err := devlog.LoadConfig(nil)
if err != nil {
	fmt.Printf("load config err: %v\n", err)
	return
}
err = devlog.CreateDeviceLogger("设备名", "设备类型", 设备id)
if err != nil {
	fmt.Printf("create device logger err: %v\n", err)
	return
}
devlog.Load("设备名").Debug("日志")
```