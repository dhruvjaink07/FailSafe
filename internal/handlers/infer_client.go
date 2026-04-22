package handlers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	pb "github.com/dhruvjaink07/failsafe/internal/proto"
)

// GRPCPredict sends an input JSON payload to the Python gRPC inference server
// and returns the raw JSON result string or an error.
func GRPCPredict(ctx context.Context, addr string, inputJSON string, timeout time.Duration) (string, error) {
	ctxDial, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctxDial, addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := pb.NewInferenceClient(conn)
	ctxCall, cancelCall := context.WithTimeout(ctx, timeout)
	defer cancelCall()

	req := &pb.PredictRequest{InputJson: inputJSON}
	res, err := client.Predict(ctxCall, req)
	if err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	return res.ResultJson, nil
}
