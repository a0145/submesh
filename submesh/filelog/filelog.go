package filelog

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type FileLog struct {
	Filename string
	lock     sync.Mutex
}

func (f *FileLog) WriteLine(source string, line string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	handle, err := os.OpenFile(f.Filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	defer handle.Close()

	// get current unixtime
	curTime := time.Now().Unix()

	_, err = handle.WriteString(fmt.Sprintf("%d,%s,%s\n", curTime, source, line))
	return err
}
