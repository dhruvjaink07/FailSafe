#!/usr/bin/env python3
"""
Read UTF-16 backend CSV, compute summaries per-endpoint and overall,
and write JSON summary plus SVG/PNG charts.
"""
import sys
from pathlib import Path
import json
import math

import pandas as pd
import matplotlib.pyplot as plt

ROOT = Path(__file__).resolve().parents[1]
RAW_CSV = ROOT / 'backend_metrics_raw.csv'
OUT_DIR = ROOT / 'experiment_results'
FIG_DIR = ROOT / 'report_package'
OUT_DIR.mkdir(exist_ok=True)
FIG_DIR.mkdir(parents=True, exist_ok=True)

def load_csv(path):
    # Attempt reading as utf-16, fall back to utf-8
    for enc in ('utf-16', 'utf-8'):
        try:
            df = pd.read_csv(path, encoding=enc)
            return df
        except Exception as e:
            last_err = e
    raise last_err


def numeric_summary(series):
    s = series.dropna().astype(float)
    if s.empty:
        return None
    return {
        'count': int(s.count()),
        'mean': float(s.mean()),
        'median': float(s.median()),
        'p95': float(s.quantile(0.95)),
        'p99': float(s.quantile(0.99)),
        'std': float(s.std()),
        'min': float(s.min()),
        'max': float(s.max())
    }


def main():
    if not RAW_CSV.exists():
        print('backend_metrics_raw.csv not found at', RAW_CSV)
        sys.exit(1)

    df = load_csv(RAW_CSV)
    # Normalize column names
    df.columns = [c.strip() for c in df.columns]

    # Find latency-like columns
    latency_cols = [c for c in df.columns if 'lat' in c.lower() or 'ms' in c.lower() or 'duration' in c.lower()]
    status_cols = [c for c in df.columns if 'status' == c.lower() or 'status' in c.lower()]

    summary = {'file': str(RAW_CSV), 'rows': len(df), 'per_endpoint': {}, 'overall': {}}

    # Per-endpoint summaries if endpoint column exists
    endpoint_col = None
    for candidate in ('endpoint', 'end_point', 'url', 'path'):
        if candidate in [c.lower() for c in df.columns]:
            # find actual column name
            endpoint_col = next(c for c in df.columns if c.lower() == candidate)
            break

    groupby_target = endpoint_col if endpoint_col is not None else None

    # Compute per-endpoint
    if groupby_target:
        groups = df.groupby(groupby_target)
        for name, g in groups:
            entry = {'rows': len(g)}
            for col in latency_cols:
                entry[col] = numeric_summary(g[col])
            # error rate
            if status_cols:
                st = g[status_cols[0]]
                errors = st.apply(lambda x: 1 if (not pd.isna(x) and int(float(x)) >= 400) else 0)
                entry['error_rate'] = float(errors.sum()) / max(1, len(g))
            summary['per_endpoint'][str(name)] = entry

    # Overall summaries
    for col in latency_cols:
        summary['overall'][col] = numeric_summary(df[col])
    if status_cols:
        st = df[status_cols[0]]
        errors = st.apply(lambda x: 1 if (not pd.isna(x) and int(float(x)) >= 400) else 0)
        summary['overall']['error_rate'] = float(errors.sum()) / max(1, len(df))

    # Write summary JSON
    out_json = OUT_DIR / 'backend_summary.json'
    with out_json.open('w', encoding='utf-8') as f:
        json.dump(summary, f, indent=2)
    print('Wrote', out_json)

    # Prepare data for plotting: combine all latency columns into single series
    all_latencies = []
    labels = []
    if latency_cols:
        for col in latency_cols:
            col_series = pd.to_numeric(df[col], errors='coerce').dropna()
            if not col_series.empty:
                all_latencies.append(col_series)
                labels.append(col)

    if all_latencies:
        plt.figure(figsize=(10,4))
        # Boxplot by column
        plt.boxplot(all_latencies, labels=labels, showfliers=False)
        plt.title('Backend latency distributions')
        plt.ylabel('ms')
        svg_path = FIG_DIR / 'backend-latency.svg'
        png_path = FIG_DIR / 'backend-latency.png'
        plt.savefig(svg_path, format='svg', bbox_inches='tight')
        plt.savefig(png_path, format='png', dpi=150, bbox_inches='tight')
        print('Wrote', svg_path)
        print('Wrote', png_path)
    else:
        print('No latency columns found to plot. Columns:', df.columns.tolist())

if __name__ == '__main__':
    main()
