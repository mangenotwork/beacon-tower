package udp

import "fmt"

const (
	SignLetterBytes    = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_+=~!@#$%^&*()"
	DefaultConnectCode = "default_connect_code"
	DefaultServersName = "servers"
	DefaultClientName  = "client"
	DefaultSecretKey   = "12345678"
)

// err

var (
	ErrNmeLengthAbove  = fmt.Errorf("名字不能超过7个长度")
	ErrDataLengthAbove = fmt.Errorf("数据大于 540个字节, 建议拆分.")
	ErrNonePacket      = fmt.Errorf("空包")
)
