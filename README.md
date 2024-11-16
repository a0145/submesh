# submesh

Basic [meshtastic](https://meshtastic.org/) info viewer, written completely in go.
Requires a MQTT server to subscribe to that has meshstatic message.
Requires the go binary and a `config.yaml` next to it

## Building

```sh
git clone https://github.com/a0145/submesh.git
cd submesh
go build -o submesh .
```

## Running

```sh
./submesh
```

## Config

Modify the MQTT server, user, pass, and topics to match what you publish meshtastic messages to

## Todo

- Filelog Management (it grows and isn't truncated)
- Multi-feeder views (allow user to switch "contexts" of different topics)
- Search
- Better DB tactic
- Pagination
- Graphics for Radios
