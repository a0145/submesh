# submesh

Basic mesh info viewer, written completely in go. Requires a MQTT server to subscribe to that has meshstatic message.
Requires the go binary and a `config.yaml` next to it

## Building

git clone https://github.com/a0145/submesh.git
cd submesh
go build -o submesh .

## Running

./submesh

## Config

Modify the MQTT server, user, pass, and topics to match what you publish meshtastic messages to
