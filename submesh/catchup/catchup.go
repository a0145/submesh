package catchup

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"submesh/submesh/contextkeys"
	"submesh/submesh/fileencoding"
	"submesh/submesh/filelog"
	"submesh/submesh/parser"

	"github.com/fxamacker/cbor/v2"
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
		dec := cbor.NewDecoder(reader)
		for {
			var logEntry fileencoding.LogEntry
			if err := dec.Decode(&logEntry); err != nil {
				if err.Error() == "EOF" {
					return
				}
				fmt.Println("error decoding cbor", err)
				break
			}

			parser.HandleRawPayload(ctx, logEntry.TimeCaptured, logEntry.Packet, true)
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
