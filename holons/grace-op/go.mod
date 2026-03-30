module github.com/organic-programming/grace-op

go 1.25.1

require (
	github.com/google/uuid v1.6.0
	github.com/jhump/protoreflect v1.18.0
	github.com/organic-programming/go-holons v0.2.1-0.20260212114054-8fbeaa095fb9
	golang.org/x/term v0.38.0
	google.golang.org/grpc v1.78.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
	nhooyr.io/websocket v1.8.17
)

require (
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0-beta.1 // indirect
	github.com/petermattis/goid v0.0.0-20260113132338-7c7de50cc741 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
)

replace github.com/organic-programming/go-holons => ../../sdk/go-holons
