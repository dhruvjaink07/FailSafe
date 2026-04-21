import argparse
import json
from concurrent import futures

import grpc
import pandas as pd

from . import infer
from .proto import infer_pb2, infer_pb2_grpc


class InferenceServicer(infer_pb2_grpc.InferenceServicer):
    def Predict(self, request, context):
        try:
            payload = json.loads(request.input_json)
            if isinstance(payload, list):
                df = pd.DataFrame(payload)
            else:
                df = pd.DataFrame([payload])

            res_df = infer.predict(df)
            result_json = res_df.to_json(orient="records", date_format="iso")
            return infer_pb2.PredictResponse(result_json=result_json)
        except Exception as e:
            return infer_pb2.PredictResponse(error=str(e))

    def BatchPredict(self, request, context):
        responses = []
        for req in request.requests:
            resp = self.Predict(req, context)
            responses.append(resp)
        return infer_pb2.BatchPredictResponse(responses=responses)

    def Health(self, request, context):
        try:
            # Do not attempt to load models here — loading the LightGBM
            # booster can invoke native code that may crash the interpreter
            # if the model file is incompatible. Instead, report service
            # availability based on module import and surface model state
            # without forcing an in-process model load.
            if getattr(infer, "_lgb_model", None) is None:
                return infer_pb2.HealthResponse(status="degraded", details="model_not_loaded")
            else:
                return infer_pb2.HealthResponse(status="ok", details="models_loaded")
        except Exception as e:
            return infer_pb2.HealthResponse(status="error", details=str(e))


def serve(host: str = "0.0.0.0", port: int = 50051, workers: int = 4):
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=workers))
    infer_pb2_grpc.add_InferenceServicer_to_server(InferenceServicer(), server)
    addr = f"{host}:{port}"
    # Try binding to the configured address; if that fails, attempt
    # localhost fallbacks to avoid platform/address-family issues on
    # Windows (IPv6/IPv4 mappings can cause add_insecure_port to fail).
    try:
        bound = server.add_insecure_port(addr)
    except Exception as e:
        print(f"[grpc] add_insecure_port exception for {addr}: {e}")
        bound = 0

    if not bound:
        fallbacks = ["127.0.0.1", "localhost", "::1"]
        for fb in fallbacks:
            fb_addr = f"{fb}:{port}"
            try:
                bound = server.add_insecure_port(fb_addr)
            except Exception as e:
                print(f"[grpc] add_insecure_port exception for {fb_addr}: {e}")
                bound = 0
            if bound:
                addr = fb_addr
                print(f"[grpc] bound to fallback address {addr}")
                break

    if not bound:
        print("[grpc] Failed to bind to any address. Set GRPC_VERBOSITY=debug and"
              " GRPC_TRACE=http to get more details. Ensure no other process is"
              " listening on the requested port.")
        raise SystemExit(1)

    print(f"[grpc] starting inference server on {addr}")
    server.start()
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        server.stop(0)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="gRPC wrapper for failsafe inference")
    parser.add_argument("--host", default="0.0.0.0")
    parser.add_argument("--port", default=50051, type=int)
    args = parser.parse_args()
    serve(host=args.host, port=args.port)
