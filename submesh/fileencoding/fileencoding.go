package fileencoding

import "time"

type LogEntry struct {
	TimeCaptured time.Time `cbor:"1,keyasint"`
	Topic        string    `cbor:"2,keyasint"`
	Packet       []byte    `cbor:"3,keyasint"`
}
