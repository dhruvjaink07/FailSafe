import psycopg2
import sys
import traceback
print("Script started")
try:
    print("Connecting...")
    conn = psycopg2.connect('postgresql://postgres:h4spRPjirm2tmiCi@db.eoaqcmjleyfqosbqxsqe.supabase.co:5432/postgres?sslmode=require', connect_timeout=10)
    print("Success")
    conn.close()
except Exception as e:
    print(f"Caught exception: {type(e).__name__}: {e}")
    traceback.print_exc()
except BaseException as e:
    print(f"Caught base exception: {type(e).__name__}: {e}")
    traceback.print_exc()
print("Script ended")
