package parser

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"submesh/submesh/contextkeys"
	"submesh/submesh/filelog"
	"submesh/submesh/state"
	"submesh/submesh/types"

	meshtastic "buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
	"github.com/eclipse/paho.golang/paho"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func generateKey(key string) ([]byte, error) {
	// Pad the key with '=' characters to ensure it's a valid base64 string
	padding := (4 - len(key)%4) % 4
	paddedKey := key + strings.Repeat("=", padding)

	// Replace '-' with '+' and '_' with '/'
	replacedKey := strings.ReplaceAll(paddedKey, "-", "+")
	replacedKey = strings.ReplaceAll(replacedKey, "_", "/")

	// Decode the base64-encoded key
	return base64.StdEncoding.DecodeString(replacedKey)
}

func generateNonce(packetId uint32, node uint32) []byte {
	packetNonce := make([]byte, 8)
	nodeNonce := make([]byte, 8)

	binary.LittleEndian.PutUint32(packetNonce, packetId)
	binary.LittleEndian.PutUint32(nodeNonce, node)

	return append(packetNonce, nodeNonce...)
}

func decode(encryptionKey []byte, encryptedData []byte, nonce []byte) (*meshtastic.Data, error) {
	var message meshtastic.Data

	ciphertext := encryptedData

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return &message, err
	}
	stream := cipher.NewCTR(block, nonce)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	err = proto.Unmarshal(plaintext, &message)
	return &message, err
}

var defaultKey []byte

func HandleRawPayload(ctx context.Context, payload []byte, catchup bool) {
	log := ctx.Value(contextkeys.Logger).(*zap.Logger)
	state := ctx.Value(contextkeys.State).(*state.State)
	var serviceEnv meshtastic.ServiceEnvelope

	err := proto.Unmarshal(payload, &serviceEnv)
	if err != nil {
		log.Error("error unmarshalling service envelope", zap.Error(err))
		return
	}
	if serviceEnv.Packet == nil {
		log.Error("service envelope missing packet")
		return
	}

	nonce := generateNonce(serviceEnv.Packet.Id, serviceEnv.Packet.From)
	if len(defaultKey) == 0 {
		key, err := generateKey("1PG7OiApB1nwvP+rz05pAQ==")
		if err != nil {
			log.Error("error generating key", zap.Error(err))
			return
		}
		defaultKey = key
	}

	var mp *meshtastic.Data
	messageSummary := types.ParsedMessage[types.MessageSummary]{
		Underlying: types.MessageSummary{
			PortNum:  0,
			PortName: "unknown",
		},
		From:         serviceEnv.Packet.From,
		To:           serviceEnv.Packet.To,
		RxSnr:        serviceEnv.Packet.RxSnr,
		HopLimit:     serviceEnv.Packet.HopLimit,
		WantAck:      serviceEnv.Packet.WantAck,
		Priority:     serviceEnv.Packet.Priority,
		HopStart:     serviceEnv.Packet.HopStart,
		PublicKey:    serviceEnv.Packet.PublicKey,
		PkiEncrypted: serviceEnv.Packet.PkiEncrypted,
		RxTime:       serviceEnv.Packet.RxTime,
		Channel:      serviceEnv.Packet.Channel,
	}
	switch serviceEnv.Packet.GetPayloadVariant().(type) {
	case *meshtastic.MeshPacket_Encrypted:
		messageSummary.Underlying.Length = len(serviceEnv.Packet.GetEncrypted())
		mp, err = decode(defaultKey, serviceEnv.Packet.GetEncrypted(), nonce)
		if err != nil {
			log.Error("error decrypting message",
				zap.Uint32("from", serviceEnv.Packet.From),
				zap.Uint32("to", serviceEnv.Packet.To),
				zap.Uint32("channel", serviceEnv.Packet.Channel),
				zap.ByteString("msg", serviceEnv.Packet.GetEncrypted()),
			)
			state.NonDecryptable.Add(
				types.ParsedMessage[int]{
					Underlying: len(serviceEnv.Packet.GetEncrypted()),
					RxTime:     serviceEnv.Packet.RxTime,
					From:       serviceEnv.Packet.From,
					To:         serviceEnv.Packet.To,
					Channel:    serviceEnv.Packet.Channel,
				},
			)
			messageSummary.Underlying.Encrypted = 1
			state.AllMessages.Add(messageSummary)
			return
		}
		messageSummary.Underlying.Encrypted = 0

	case *meshtastic.MeshPacket_Decoded:
		break
		// skip these as they seem to be non-sensical and duplicated from the encrypted stream

		// mp = serviceEnv.Packet.GetDecoded()
		// messageSummary.Underlying.Encrypted = -1
		// messageSummary.Underlying.Length = len(mp.Payload)

	}

	if mp == nil {
		log.Error("no message payload")
		return
	}

	messageSummary.Underlying.PortName = mp.Portnum.String()
	messageSummary.Underlying.PortNum = uint32(mp.Portnum.Number())

	log = log.With(zap.Any("portnum", mp.Portnum))
	switch mp.Portnum {
	case meshtastic.PortNum_TELEMETRY_APP:
		var data meshtastic.Telemetry
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling telemetry app", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		switch data.GetVariant().(type) {
		case *meshtastic.Telemetry_AirQualityMetrics:
			if !catchup {
				log.Info("received Air Quality telemetry", zap.Any("data", data.GetAirQualityMetrics()))
			}
		case *meshtastic.Telemetry_DeviceMetrics:
			if !catchup {
				log.Info("received Device Metrics telemetry", zap.Any("data", data.GetDeviceMetrics()))
			}
		case *meshtastic.Telemetry_EnvironmentMetrics:
			if !catchup {
				log.Info("received Environment Metrics telemetry", zap.Any("data", data.GetEnvironmentMetrics()))
			}
		case *meshtastic.Telemetry_HealthMetrics:
			if !catchup {
				log.Info("received Health Metrics telemetry", zap.Any("data", data.GetHealthMetrics()))
			}
		case *meshtastic.Telemetry_LocalStats:
			if !catchup {
				log.Info("received Local Stats telemetry", zap.Any("data", data.GetLocalStats()))
			}
		case *meshtastic.Telemetry_PowerMetrics:
			if !catchup {
				log.Info("received Power Metrics telemetry", zap.Any("data", data.GetPowerMetrics()))
			}
		default:
			log.Error("unknown telemetry app message", zap.Any("variant", data.GetVariant()))
		}
		state.Telemetry.Add(
			types.ParsedMessage[meshtastic.Telemetry]{
				Underlying: data,
				RxTime:     serviceEnv.Packet.RxTime,
				From:       serviceEnv.Packet.From,
				To:         serviceEnv.Packet.To,
				Channel:    serviceEnv.Packet.Channel,
			}, fmt.Sprintf("%d", serviceEnv.Packet.From))
	case meshtastic.PortNum_NEIGHBORINFO_APP:
		var data meshtastic.NeighborInfo
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		if !catchup {
			log.Info("received message", zap.String("data", data.String()))
		}
		state.Neighbors.Add(
			types.ParsedMessage[meshtastic.NeighborInfo]{
				Underlying: data,
				RxTime:     serviceEnv.Packet.RxTime,
				From:       serviceEnv.Packet.From,
				To:         serviceEnv.Packet.To,
				Channel:    serviceEnv.Packet.Channel,
			}, fmt.Sprintf("%d", data.NodeId))
	case meshtastic.PortNum_NODEINFO_APP:
		var data meshtastic.User
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		if !catchup {
			log.Info("received message", zap.String("data", data.String()))
		}
		state.Users.Add(
			types.ParsedMessage[meshtastic.User]{
				Underlying: data,
				RxTime:     serviceEnv.Packet.RxTime,
				From:       serviceEnv.Packet.From,
				To:         serviceEnv.Packet.To,
				Channel:    serviceEnv.Packet.Channel,
			},
			fmt.Sprintf("%d", serviceEnv.Packet.From), data.Id, data.ShortName)
	case meshtastic.PortNum_POSITION_APP:
		var data meshtastic.Position
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		if !catchup {
			log.Info("received message", zap.String("data", data.String()))
		}
		state.Positions.Add(
			types.ParsedMessage[meshtastic.Position]{
				Underlying: data,
				RxTime:     serviceEnv.Packet.RxTime,
				From:       serviceEnv.Packet.From,
				To:         serviceEnv.Packet.To,
				Channel:    serviceEnv.Packet.Channel,
			}, fmt.Sprintf("%d", serviceEnv.Packet.From))
	case meshtastic.PortNum_TEXT_MESSAGE_APP:
		if !catchup {
			log.Info("received text message", zap.String("data", string(mp.Payload)))
		}
		messageSummary.Underlying.Summary = string(mp.Payload)
		state.Chats.Add(
			types.ParsedMessage[string]{
				Underlying: string(mp.Payload),
				RxTime:     serviceEnv.Packet.RxTime,
				From:       serviceEnv.Packet.From,
				To:         serviceEnv.Packet.To,
				Channel:    serviceEnv.Packet.Channel,
			}, "last")
	case meshtastic.PortNum_TRACEROUTE_APP:
		var data meshtastic.RouteDiscovery
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		if !catchup {
			log.Info("received message", zap.String("data", data.String()))
		}
		state.Traceroutes.Add(
			types.ParsedMessage[meshtastic.RouteDiscovery]{
				Underlying: data,
				RxTime:     serviceEnv.Packet.RxTime,
				From:       serviceEnv.Packet.From,
				To:         serviceEnv.Packet.To,
				Channel:    serviceEnv.Packet.Channel,
			}, fmt.Sprintf("%d", serviceEnv.Packet.From))
	case meshtastic.PortNum_MAP_REPORT_APP:
		var data meshtastic.MapReport
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		if !catchup {
			log.Info("received message", zap.String("data", data.String()))
		}
	case meshtastic.PortNum_ROUTING_APP:
		var data meshtastic.Routing
		err = proto.Unmarshal(mp.Payload, &data)
		if err != nil {
			log.Error("error unmarshalling", zap.Error(err))
			return
		}
		messageSummary.Underlying.Summary = protojson.Format(&data)
		if !catchup {
			log.Info("received message", zap.String("data", data.String()))
		}
	default:
		log.Error("unknown port number")
	}
	state.AllMessages.Add(messageSummary)

}
func HandleMQTTMessage(ctx context.Context, pr paho.PublishReceived) {

	log := ctx.Value(contextkeys.Logger).(*zap.Logger).With(zap.String("module", "parser"), zap.String("topic", pr.Packet.Topic))
	if strings.Contains(pr.Packet.Topic, "/json/") {
		// json log
		log.Info("received json message")
		return
	}
	// save to bytelog
	ctx.Value(contextkeys.RAWFileLogger).(*filelog.FileLog).Write(
		pr.Packet.Topic, pr.Packet.Payload,
	)

	HandleRawPayload(ctx, pr.Packet.Payload, false)
}
