package catchup

import (
	"bufio"
	"context"
	"encoding/base64"
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
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			// split

			parts := strings.Split(scanner.Text(), ",")
			var decoded []byte
			if len(parts) == 1 {
				// recover bytes
				decoded, err = base64.StdEncoding.DecodeString(scanner.Text())
				if err != nil {
					logger.Error("error decoding base64", zap.Error(err))
				}
			} else if len(parts) == 3 {
				_ = parts[0] // time
				_ = parts[1] // topic
				decoded, err = base64.StdEncoding.DecodeString(parts[2])
				if err != nil {
					logger.Error("error decoding base64", zap.Error(err))
				}
			} else {
				logger.Error("error parsing line", zap.String("line", scanner.Text()))
			}
			parser.HandleRawPayload(ctx, decoded)
		}
	}
}
