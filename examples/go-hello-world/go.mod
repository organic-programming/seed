module github.com/organic-programming/examples/hello-world

go 1.25.1

require (
	github.com/organic-programming/go-holons v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.78.0
	google.golang.org/protobuf v1.36.10
	nhooyr.io/websocket v1.8.17
)

require (
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
)

replace github.com/organic-programming/go-holons => ../../sdk/go-holons
