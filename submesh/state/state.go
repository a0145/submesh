package state

import (
	"submesh/submesh/types"
	"time"

	"buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"
)

type State struct {
	Users          HistoricalWithLastByPK[meshtastic.User]
	Telemetry      HistoricalWithLastByPK[meshtastic.Telemetry]
	Chats          HistoricalWithLastByPK[string]
	NonDecryptable HistoricalWithLastByPK[int]
	AllMessages    HistoricalWithLastByPK[types.MessageSummary]
	Neighbors      HistoricalWithLastByPK[meshtastic.NeighborInfo]
	Positions      HistoricalWithLastByPK[meshtastic.Position]
	Traceroutes    HistoricalWithLastByPK[meshtastic.RouteDiscovery]
	ProcessedHash  map[string]time.Time
}

func NewState() *State {
	return &State{
		Users:          NewHistoricalWithLastByPK[meshtastic.User](),
		Telemetry:      NewHistoricalWithLastByPK[meshtastic.Telemetry](),
		Chats:          NewHistoricalWithLastByPK[string](),
		NonDecryptable: NewHistoricalWithLastByPK[int](),
		AllMessages:    NewHistoricalWithLastByPK[types.MessageSummary](),
		Neighbors:      NewHistoricalWithLastByPK[meshtastic.NeighborInfo](),
		Positions:      NewHistoricalWithLastByPK[meshtastic.Position](),
		Traceroutes:    NewHistoricalWithLastByPK[meshtastic.RouteDiscovery](),
		ProcessedHash:  make(map[string]time.Time),
	}
}
