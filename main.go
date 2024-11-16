package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"submesh/submesh/catchup"
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

const AppVersion = "0.0.3"

func doConfig() {
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
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// setup config
	doConfig()

	// setup logger
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zap.InfoLevel)
	config := zap.NewProductionConfig()
	config.Level = atomicLevel

	logger, _ := config.Build()

	defer logger.Sync()
	b64log := "log.b64lines"

	if viper.GetBool("submesh.production") {
		logger.Info("running in production mode")
		b64log = "log_prod.b64lines"
	}

	// setup context
	ctx = context.WithValue(ctx, contextkeys.RAWFileLogger, &filelog.FileLog{Filename: b64log})
	ctx = context.WithValue(ctx, contextkeys.Logger, logger)
	ctx = context.WithValue(ctx, contextkeys.State, state.NewState())
	ctx = context.WithValue(ctx, contextkeys.AtomicLevel, &atomicLevel)
	ctx = context.WithValue(ctx, contextkeys.AppVersion, AppVersion)

	// catch up
	catchup.CatchUp(ctx)

	// subscribe to mqtt
	go doMqtt(ctx)

	// start webserver
	go web.StartServer(ctx)

	<-ctx.Done()

	logger.Warn("signal caught, exiting")
}

func doMqtt(ctx context.Context) {
	logger := ctx.Value(contextkeys.Logger).(*zap.Logger)

	u, err := url.Parse(fmt.Sprintf("mqtt://%s:%d", viper.GetString("mqtt.host"), viper.GetInt("mqtt.port")))
	if viper.GetString("mqtt.username") != "" {
		u.User = url.UserPassword(viper.GetString("mqtt.username"), viper.GetString("mqtt.password"))
	}
	if err != nil {
		panic(err)
	}

	topics := viper.GetStringSlice("mqtt.topics")
	logger.Info("subscribing to topics", zap.Strings("topics", topics))
	mqtt.MQTTConnectAndListen(ctx, topics, u, parser.HandleMQTTMessage)
}
