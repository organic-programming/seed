module github.com/organic-programming/examples/web-hello-world

go 1.24.0

require github.com/organic-programming/go-holons v0.0.0

require (
	google.golang.org/grpc v1.78.0 // indirect
	nhooyr.io/websocket v1.8.17 // indirect
)

replace github.com/organic-programming/go-holons => ../../../sdk/go-holons
