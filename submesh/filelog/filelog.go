package filelog

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

type FileLog struct {
	lumberjack *lumberjack.Logger
}

func NewFileLog(filename string) *FileLog {
	return &FileLog{
		lumberjack: &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    viper.GetInt("submesh.db.max_megs"),
			MaxBackups: viper.GetInt("submesh.db.max_backups"),
			MaxAge:     viper.GetInt("submesh.db.max_age"),
			Compress:   true,
		},
	}
}
func (f *FileLog) Filename() string {
	return f.lumberjack.Filename
}

func (f *FileLog) Close() {
	f.lumberjack.Close()
}

func (f *FileLog) WriteLine(source string, line string) error {
	// get current unixtime
	curTime := time.Now().Unix()
	_, err := f.lumberjack.Write([]byte(fmt.Sprintf("%d,%s,%s\n", curTime, source, line)))
	return err
}
