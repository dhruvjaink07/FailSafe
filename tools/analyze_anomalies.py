#!/usr/bin/env python3
import json
import csv
from pathlib import Path

ROOT = Path('d:/FailSafe/deployments/docker/experiments/results')
OUT_JSON = ROOT / 'anomalies.json'
OUT_CSV = ROOT / 'anomalies.csv'

def load_json(p):
    try:
        return json.loads(p.read_text(encoding='utf-8'))
    except Exception:
        try:
            return json.loads(p.read_text(encoding='utf-8-sig'))
        except Exception:
            return None

def find_start_for(metrics_path):
    base = metrics_path.with_name(metrics_path.name.replace('-metrics.json','-start.json'))
    if base.exists():
        return load_json(base)
    # try nearby directories
    return None

def analyze():
    anomalies = []
    for f in ROOT.rglob('*-metrics.json'):
        metrics = load_json(f)
        if not metrics: 
            continue
        start = find_start_for(f)
        payload = None
        fault = None
        if start and isinstance(start, dict):
            payload = start.get('payload') or start.get('experiment') or start
            fault = payload.get('faultType') if isinstance(payload, dict) else None

        # some metric files may use different keys; try common alternatives
        endpoints = metrics.get('endpoints') or metrics.get('endpoints_metrics') or metrics.get('endpoint_metrics') or {}
        for endpoint, data in endpoints.items():
            degraded = bool(data.get('degraded'))
            stability = data.get('stability_score', 100)
            errors = data.get('errors', {})
            error_rate = errors.get('rate_percent', 0)
            if degraded and (stability < 80 or error_rate > 0):
                category = 'expected_destructive' if fault == 'kill' else 'unexpected'
                anomalies.append({
                    'experiment_file': str(f.relative_to(ROOT)),
                    'endpoint': endpoint,
                    'faultType': fault,
                    'degraded': degraded,
                    'stability_score': stability,
                    'error_rate_percent': error_rate,
                    'category': category
                })

    # write outputs
    OUT_JSON.write_text(json.dumps(anomalies, indent=2), encoding='utf-8')
    with OUT_CSV.open('w', newline='', encoding='utf-8') as fh:
        writer = csv.DictWriter(fh, fieldnames=['experiment_file','endpoint','faultType','degraded','stability_score','error_rate_percent','category'])
        writer.writeheader()
        for a in anomalies:
            writer.writerow(a)

if __name__ == '__main__':
    analyze()
    print('Wrote', OUT_JSON, OUT_CSV)
