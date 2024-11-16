package mqtt

import (
	"context"
	"fmt"
	"net/url"
	"submesh/submesh/contextkeys"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const clientID = "SubMesh"

func MQTTConnectAndListen(ctx context.Context, topics []string, u *url.URL, handleMessage func(ctx context.Context, msg paho.PublishReceived)) {
	log := ctx.Value(contextkeys.Logger).(*zap.Logger).With(zap.String("module", "mqtt"))
	subOptions := []paho.SubscribeOptions{}
	for _, topic := range topics {
		subOptions = append(subOptions, paho.SubscribeOptions{
			Topic: topic,
			QoS:   1,
		})
	}
	cliCfg := autopaho.ClientConfig{
		ServerUrls: []*url.URL{u},
		KeepAlive:  20, // Keepalive message should be sent every 20 seconds
		// CleanStartOnInitialConnection defaults to false. Setting this to true will clear the session on the first connection.
		CleanStartOnInitialConnection: false,
		// SessionExpiryInterval - Seconds that a session will survive after disconnection.
		// It is important to set this because otherwise, any queued messages will be lost if the connection drops and
		// the server will not queue messages while it is down. The specific setting will depend upon your needs
		// (60 = 1 minute, 3600 = 1 hour, 86400 = one day, 0xFFFFFFFE = 136 years, 0xFFFFFFFF = don't expire)
		SessionExpiryInterval: 60,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			log.Info("connection up")
			// Subscribing in the OnConnectionUp callback is recommended (ensures the subscription is reestablished if
			// the connection drops)
			if _, err := cm.Subscribe(context.Background(), &paho.Subscribe{
				Subscriptions: subOptions,
			}); err != nil {
				log.Error("failed to subscribe", zap.Error(err))
			}
			log.Info("mqtt subscription made")
		},
		OnConnectError: func(err error) {
			log.Error("error whilst attempting connection", zap.Error(err))
		},
		ClientConfig: paho.ClientConfig{
			ClientID: fmt.Sprintf("%s_%s", clientID, uuid.NewString()),
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){func(msg paho.PublishReceived) (bool, error) {
				handleMessage(ctx, msg)
				return true, nil
			}},
			OnClientError: func(err error) { log.Error("client error", zap.Error(err)) },
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					log.Warn("server requested disconnect", zap.String("reason", d.Properties.ReasonString))
				} else {
					log.Warn("server requested disconnect", zap.Int("reason", int(d.ReasonCode)))
				}
			},
		},
	}

	if u.User != nil {
		cliCfg.ConnectUsername = u.User.Username()
		log.Info("using provided username")
	}
	if password, ok := u.User.Password(); ok {
		cliCfg.ConnectPassword = []byte(password)
		log.Info("using provided password")

	}
	c, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		log.Fatal("failed to create connection", zap.Error(err))
	}

	// Wait for the connection to come up
	if err = c.AwaitConnection(ctx); err != nil {
		log.Fatal("failed to connect", zap.Error(err))
	}

	<-c.Done()
}
