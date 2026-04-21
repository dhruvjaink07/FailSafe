import os
import argparse
import json
import grpc

from internal.Prod.proto import infer_pb2, infer_pb2_grpc


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--addr", default=os.getenv("PYTHON_GRPC_ADDR", "127.0.0.1:50051"))
    p.add_argument("--json", help="JSON payload string to send")
    p.add_argument("--file", help="Path to JSON file containing payload (list or object)")
    args = p.parse_args()

    if args.file:
        with open(args.file, "r", encoding="utf-8") as f:
            payload = json.load(f)
    elif args.json:
        payload = json.loads(args.json)
    else:
        # default sample payload
        payload = {"id": "test", "fault_intensity": 0.5}

    # The server expects a JSON string in `input_json`
    input_json = json.dumps(payload)

    channel = grpc.insecure_channel(args.addr)
    try:
        grpc.channel_ready_future(channel).result(timeout=5)
    except Exception as e:
        print(f"Failed to connect to {args.addr}: {e}")
        return

    client = infer_pb2_grpc.InferenceStub(channel)
    try:
        req = infer_pb2.PredictRequest(input_json=input_json)
        resp = client.Predict(req, timeout=10)
        if resp.error:
            print(f"error: {resp.error}")
        else:
            try:
                parsed = json.loads(resp.result_json)
                print(json.dumps(parsed, indent=2))
            except Exception:
                print(resp.result_json)
    except Exception as e:
        print(f"Predict RPC failed: {e}")


if __name__ == "__main__":
    main()
