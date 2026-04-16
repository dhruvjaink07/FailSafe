Backend metrics extraction (consolidated copy)

Run the extraction script to compute numeric summaries and generate charts.

1. Create a virtualenv (optional):

```bash
python -m venv .venv
source .venv/bin/activate   # or .\.venv\Scripts\activate on Windows
```

2. Install requirements:

```bash
pip install -r report_package/requirements.txt
```

3. Run the script (will read `backend_metrics_raw.csv` and write `experiment_results/backend_summary.json` and `report_package/backend-latency.svg`/`.png`):

```bash
python report_package/extract_backend_metrics.py
```

Files produced:
- `experiment_results/backend_summary.json` — per-endpoint and overall numeric summaries
- `report_package/backend-latency.svg` — SVG boxplot
- `report_package/backend-latency.png` — PNG export
