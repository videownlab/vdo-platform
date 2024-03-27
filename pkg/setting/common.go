package setting

import (
	"path"
	"time"
)

const RunModeLive = "release"
const RunModeDev = "debug"

type ServerSettingS struct {
	RunMode      string
	HttpPort     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func (t *ServerSettingS) IsDebugMode() bool {
	return t.RunMode == RunModeDev
}

type AppSettingS struct {
	DefaultPageSize int
	MaxPageSize     int
	LogSavePath     string
	LogFileName     string
	LogFileExt      string
	CmpHttpUrl      string
	JwtSecret       string
	OutputAuthCode  bool
	// by second
	JwtDuration int
	Username    string
	Password    string

	Limiter struct {
		IsOpen bool
		Count  int64
		Gap    int32
	}
}

func (t *AppSettingS) FullLogFilePath() string {
	return path.Join(t.LogSavePath, t.LogFileName+t.LogFileExt)
}
