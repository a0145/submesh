package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"submesh/submesh/contextkeys"
	"submesh/submesh/filelog"
	"submesh/submesh/mqtt"
	"submesh/submesh/parser"
	"submesh/submesh/state"
	"submesh/submesh/web"
	"syscall"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// LoadConfig
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/submesh/")
	viper.AddConfigPath("$HOME/.submesh")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	viper.SetDefault("web.port", 8080)
	viper.SetDefault("mqtt.host", "localhost")
	viper.SetDefault("mqtt.port", 1883)
	viper.SetDefault("mqtt.username", "")
	viper.SetDefault("mqtt.password", "")
	viper.SetDefault("mqtt.topics", []string{})
	viper.SetDefault("submesh.production", false)

	// build url from config

	u, err := url.Parse(fmt.Sprintf("mqtt://%s:%d", viper.GetString("mqtt.host"), viper.GetInt("mqtt.port")))
	if viper.GetString("mqtt.username") != "" {
		u.User = url.UserPassword(viper.GetString("mqtt.username"), viper.GetString("mqtt.password"))
	}
	if err != nil {
		panic(err)
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()
	b64log := "log.b64lines"

	if viper.GetBool("submesh.production") {
		logger.Info("running in production mode")
		b64log = "log_prod.b64lines"
	}

	// add logger to context
	ctx = context.WithValue(ctx, contextkeys.RAWFileLogger, &filelog.FileLog{Filename: b64log})
	ctx = context.WithValue(ctx, contextkeys.Logger, logger)
	ctx = context.WithValue(ctx, contextkeys.State, state.NewState())

	if _, err := os.Stat(b64log); err == nil {
		// open file and read line by line
		file, err := os.Open(b64log)
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
	go web.StartServer(ctx)

	for _, topic := range viper.GetStringSlice("mqtt.topics") {
		logger.Info("subscribing to topic", zap.String("topic", topic))
		mqtt.MQTTConnect(ctx, topic, u, parser.HandleMQTTMessage)
	}

	fmt.Println("signal caught - exiting")
}
