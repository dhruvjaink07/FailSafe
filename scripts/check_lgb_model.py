#!/usr/bin/env python3
import sys
import traceback
from pathlib import Path

MODEL_PATH = Path("internal/Prod/models/lgb_model.txt")

def main():
    print("Python:", sys.executable)
    try:
        import lightgbm as lgb
        print("lightgbm:", lgb.__version__)
    except Exception as e:
        print("Failed to import lightgbm:", e)
        traceback.print_exc()
        sys.exit(2)

    if not MODEL_PATH.exists():
        print("Model file not found:", MODEL_PATH)
        sys.exit(3)

    # show header
    with MODEL_PATH.open('r', encoding='utf-8', errors='replace') as f:
        header = [next(f) for _ in range(8)]
    print("--- model header ---")
    for i, ln in enumerate(header, start=1):
        print(f"{i}: {ln.rstrip()}")
    print("--- end header ---")

    # Try loading via model_file
    try:
        booster = lgb.Booster(model_file=str(MODEL_PATH))
        print("OK: model loaded via model_file")
    except Exception as e:
        print("LOAD ERROR (model_file):", e)
        traceback.print_exc()

    # Try loading via model_str
    try:
        s = MODEL_PATH.read_text(encoding='utf-8', errors='replace')
        booster2 = lgb.Booster(model_str=s)
        print("OK: model loaded via model_str")
    except Exception as e:
        print("LOAD ERROR (model_str):", e)
        traceback.print_exc()

    # exit non-zero if both failed
    try:
        _ = booster
        return
    except NameError:
        try:
            _ = booster2
            return
        except NameError:
            sys.exit(1)

if __name__ == '__main__':
    main()
