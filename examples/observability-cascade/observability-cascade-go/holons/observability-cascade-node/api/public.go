package api

import (
	"context"

	pb "observability-cascade-node-go/gen/go/relay/v1"
	"observability-cascade-node-go/internal"
)

// Tick emits one cascade tick through the in-process implementation.
func Tick(req *pb.TickRequest) (*pb.TickResponse, error) {
	return (&internal.Server{}).Tick(context.Background(), req)
}
