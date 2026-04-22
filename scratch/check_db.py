import os
import sys
import traceback
import psycopg2
from psycopg2.extras import RealDictCursor

# Manual .env load
env_vars = {}
try:
    with open(".env", "r") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env_vars[k.strip()] = v.strip()
except:
    pass

# Prefer explicit Postgres connection params (DB_HOST / DB_USER / DB_PASSWORD / DB_NAME)
db_host = env_vars.get("DB_HOST") or os.getenv("DB_HOST")
db_user = env_vars.get("DB_USER") or os.getenv("DB_USER")
db_password = env_vars.get("DB_PASSWORD") or os.getenv("DB_PASSWORD")
db_name = env_vars.get("DB_NAME") or os.getenv("DB_NAME")
db_port = env_vars.get("DB_PORT") or os.getenv("DB_PORT") or "5432"
db_ssl = env_vars.get("DB_SSLMODE") or os.getenv("DB_SSLMODE")

if db_host and db_user and db_password and db_name:
    ssl_query = f"?sslmode={db_ssl}" if db_ssl else ""
    db_url = f"postgresql://{db_user}:{db_password}@{db_host}:{db_port}/{db_name}{ssl_query}"
    source = "DB_HOST/DB_*"
else:
    db_url = env_vars.get("DB_URL") or os.getenv("DB_URL")
    source = "DB_URL"

if not db_url:
    print("No DB configuration found. Set DB_URL or DB_HOST/DB_USER/DB_PASSWORD/DB_NAME in .env or environment.")
    sys.exit(1)

print(f"Testing DB connection to ({source}): {db_url[:80]}...")

try:
    print("Attempting psycopg2.connect()...")
    conn = psycopg2.connect(db_url, cursor_factory=RealDictCursor)
    print("Connected — running test queries")
    cur = conn.cursor()

    try:
        cur.execute("SELECT COUNT(*) FROM experiments")
        exp_count = cur.fetchone()["count"]
        print(f"Experiments count: {exp_count}")
    except Exception as qerr:
        print("Query error (experiments):")
        traceback.print_exc()

    try:
        cur.execute("SELECT COUNT(*) FROM backend_status_metrics")
        metrics_count = cur.fetchone()["count"]
        print(f"Metrics count: {metrics_count}")
    except Exception as qerr:
        print("Query error (backend_status_metrics):")
        traceback.print_exc()

    try:
        cur.close()
        conn.close()
    except Exception:
        pass
except Exception:
    print("DB Error — full traceback:")
    traceback.print_exc()
    sys.exit(1)
