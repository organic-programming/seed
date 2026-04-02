package server

import (
	codexloopsv1 "github.com/organic-programming/codex-loops/gen/go/v1"
	holonserve "github.com/organic-programming/go-holons/pkg/serve"
	"google.golang.org/grpc"
)

func ListenAndServe(listenURIs []string, reflect bool, handler codexloopsv1.CodexLoopsServiceServer) error {
	if len(listenURIs) == 0 {
		listenURIs = []string{holonserve.ParseOptions(nil).ListenURI}
	}
	return holonserve.RunWithOptions(listenURIs[0], func(server *grpc.Server) {
		codexloopsv1.RegisterCodexLoopsServiceServer(server, handler)
	}, reflect, listenURIs[1:]...)
}
