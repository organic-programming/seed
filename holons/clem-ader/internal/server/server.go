package server

import (
	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
	holonserve "github.com/organic-programming/go-holons/pkg/serve"

	"google.golang.org/grpc"
)

// ListenAndServe exposes the Ader RPC surface on the requested URIs.
func ListenAndServe(listenURIs []string, reflect bool, handler aderv1.AderServiceServer) error {
	if len(listenURIs) == 0 {
		listenURIs = []string{holonserve.ParseOptions(nil).ListenURI}
	}
	return holonserve.RunWithOptions(listenURIs[0], func(server *grpc.Server) {
		aderv1.RegisterAderServiceServer(server, handler)
	}, reflect, listenURIs[1:]...)
}
