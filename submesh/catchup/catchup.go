package catchup

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"submesh/submesh/contextkeys"
	"submesh/submesh/filelog"
	"submesh/submesh/parser"

	"go.uber.org/zap"
)

func CatchUp(ctx context.Context) {
	filename := ctx.Value(contextkeys.RAWFileLogger).(*filelog.FileLog).Filename()
	logger := ctx.Value(contextkeys.Logger).(*zap.Logger)
	atomicLevel := ctx.Value(contextkeys.AtomicLevel).(*zap.AtomicLevel)
	prevLevel := atomicLevel.Level()
	defer atomicLevel.SetLevel(prevLevel)
	// Mute Logger for Info
	atomicLevel.SetLevel(zap.PanicLevel)

	if _, err := os.Stat(filename); err == nil {
		// open file and read line by line

		file, err := os.Open(filename)
		if err != nil {
			logger.Fatal("error opening file", zap.Error(err))
		}
		defer file.Close()

		count := 0
		reader := bufio.NewReader(file)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			// split

			index := strings.LastIndexByte(line, ',')

			var decoded []byte
			if index == -1 {
				// recover bytes
				decoded, err = base64.StdEncoding.DecodeString(line)
				if err != nil {
					logger.Error("error decoding base64", zap.Error(err))
				}
			} else {

				// time
				// topic
				// line[index+1:]
				decoded, err = base64.StdEncoding.DecodeString(line[index+1:])
				if err != nil {
					logger.Error("error decoding base64", zap.Error(err))
				}
			}
			parser.HandleRawPayload(ctx, decoded)
			count += 1
			if count%10000 == 0 {
				fmt.Println("catching up", count)
			}
			if ctx.Err() != nil {
				break
			}
		}
	}
}
