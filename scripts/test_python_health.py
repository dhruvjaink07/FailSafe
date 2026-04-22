import os
import argparse
import grpc

from internal.Prod.proto import infer_pb2, infer_pb2_grpc


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--addr", default=os.getenv("PYTHON_GRPC_ADDR", "127.0.0.1:50051"), help="gRPC server address")
    args = p.parse_args()

    channel = grpc.insecure_channel(args.addr)
    try:
        grpc.channel_ready_future(channel).result(timeout=5)
    except Exception as e:
        print(f"Failed to connect to {args.addr}: {e}")
        return

    client = infer_pb2_grpc.InferenceStub(channel)
    try:
        resp = client.Health(infer_pb2.HealthRequest(), timeout=5)
        print(f"status: {resp.status}\ndetails: {resp.details}")
    except Exception as e:
        print(f"Health RPC failed: {e}")


if __name__ == "__main__":
    main()
