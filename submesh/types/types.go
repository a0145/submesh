package types

import "buf.build/gen/go/meshtastic/protobufs/protocolbuffers/go/meshtastic"

type ParsedMessage[T any] struct {
	Underlying   T
	RxTime       uint32
	From         uint32
	To           uint32
	Id           uint32
	RxSnr        float32
	HopLimit     uint32
	WantAck      bool
	Priority     meshtastic.MeshPacket_Priority
	HopStart     uint32
	PublicKey    []byte
	PkiEncrypted bool
	Channel      uint32
}

type MessageSummary struct {
	PortNum   uint32
	PortName  string
	Length    int
	Encrypted int
	Summary   string
}
