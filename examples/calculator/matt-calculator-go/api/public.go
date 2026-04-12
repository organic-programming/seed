package api

import (
	"fmt"

	pb "matt-calculator-go/gen/go/calculator/v1"
	"matt-calculator-go/internal"
)

var globalAcc = &internal.Accumulator{}

func Set(req *pb.SetRequest) (*pb.CalcResponse, error) {
	result := globalAcc.Set(req.GetValue())
	return &pb.CalcResponse{
		Result:     result,
		Expression: fmt.Sprintf("set → %s", formatFloat(result)),
	}, nil
}

func Add(req *pb.AddSubRequest) (*pb.CalcResponse, error) {
	prev, next := globalAcc.Add(req.GetValue())
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s + %s = %s", formatFloat(prev), formatFloat(req.GetValue()), formatFloat(next)),
	}, nil
}

func Subtract(req *pb.AddSubRequest) (*pb.CalcResponse, error) {
	prev, next := globalAcc.Subtract(req.GetValue())
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s - %s = %s", formatFloat(prev), formatFloat(req.GetValue()), formatFloat(next)),
	}, nil
}

func Multiply(req *pb.MulDivRequest) (*pb.CalcResponse, error) {
	prev, next := globalAcc.Multiply(req.GetBy())
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s × %s = %s", formatFloat(prev), formatFloat(req.GetBy()), formatFloat(next)),
	}, nil
}

func Divide(req *pb.MulDivRequest) (*pb.CalcResponse, error) {
	prev, next, err := globalAcc.Divide(req.GetBy())
	if err != nil {
		return nil, err
	}
	return &pb.CalcResponse{
		Result:     next,
		Expression: fmt.Sprintf("%s / %s = %s", formatFloat(prev), formatFloat(req.GetBy()), formatFloat(next)),
	}, nil
}

func formatFloat(value float64) string {
	return internal.FormatFloat(value)
}
