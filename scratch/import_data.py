import psycopg2
import os
import re
import io
from urllib.parse import urlparse

# Get DB_URL from environment or .env
DB_URL = "postgresql://failsafe_dev:jCRXjk6GaoAAKCt4xOSn7FWsUeSYUozl@dpg-d793pgoule4c73afpbsg-a.oregon-postgres.render.com:5432/failsafe?sslmode=require"
DUMP_PATH = r"d:\Failsafe\Risk\failsafe_dump.sql\failsafe_dump.sql"

def import_dump():
    print(f"Connecting to {urlparse(DB_URL).hostname}...")
    conn = psycopg2.connect(DB_URL)
    conn.autocommit = False
    cur = conn.cursor()

    with open(DUMP_PATH, 'r', encoding='utf-8') as f:
        lines = f.readlines()

    print(f"Read {len(lines)} lines from dump.")

    # Skip administrative lines
    skip_patterns = [
        r"^CREATE ROLE",
        r"^ALTER ROLE",
        r"^CREATE DATABASE",
        r"^ALTER DATABASE",
        r"^\\connect",
        r"^\\restrict",
        r"^\\unrestrict",
    ]

    TABLES = [
        "android_experiment_report", "android_experiment_summary", "android_experiments",
        "android_metrics_raw", "android_status_metrics", "api_keys",
        "backend_experiments", "backend_metrics_raw", "backend_status_metrics",
        "experiment_summary", "experiments", "frontend_experiments",
        "frontend_metrics_raw", "frontend_status_metrics", "metrics_aggregated",
        "metrics_raw", "users"
    ]

    print("Detecting existing tables...")
    cur.execute("SELECT table_name FROM information_schema.tables WHERE table_schema='public';")
    existing_tables = [r[0] for r in cur.fetchall()]
    print(f"Found {len(existing_tables)} tables.")

    tables_to_drop = [t for t in TABLES if t in existing_tables]
    if tables_to_drop:
        print(f"Dropping {len(tables_to_drop)} tables...")
        drop_stmt = f"DROP TABLE {', '.join(['public.' + t for t in tables_to_drop])} CASCADE;"
        try:
            cur.execute(drop_stmt)
            conn.commit()
            print("Drop successful.")
        except Exception as e:
            print(f"Drop error: {e}")
            conn.rollback()
    else:
        print("No tables to drop.")

    current_stmt = []
    in_copy = False
    copy_stmt = ""
    copy_data = []

    for i, line in enumerate(lines):
        # Skip administrative commands
        if any(re.match(p, line) for p in skip_patterns):
            continue
        
        if line.startswith("COPY "):
            # If we were accumulating a statement, flush it? (Shouldn't happen in pg_dump)
            in_copy = True
            copy_stmt = line.strip().replace("FROM stdin", "FROM STDIN")
            copy_data = []
            # print(f"Starting COPY: {copy_stmt[:50]}...")
            continue
        
        if in_copy:
            if line.strip() == r"\.":
                in_copy = False
                data_str = "".join(copy_data)
                cur.copy_expert(copy_stmt, io.StringIO(data_str))
                # print(f"Finished COPY ({len(copy_data)} rows).")
            else:
                copy_data.append(line)
            continue

        # Accumulate regular SQL command
        clean_line = line.split('--')[0].strip()
        if not clean_line:
            continue
            
        # Skip OWNER TO commands
        if "OWNER TO" in line:
            continue

        current_stmt.append(line)
        if clean_line.endswith(";"):
            stmt = "".join(current_stmt)
            try:
                # Handle SET search_path specifically
                if "set_config('search_path'" in stmt:
                    cur.execute("SET search_path TO public;")
                else:
                    cur.execute(stmt)
            except Exception as e:
                print(f"Error at line {i+1}: {e}")
                conn.rollback()
            current_stmt = []

    conn.commit()
    print("Import completed successfully!")
    cur.close()
    conn.close()

if __name__ == "__main__":
    import_dump()
