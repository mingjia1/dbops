import sqlite3
db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
with db:
    cols = db.execute("PRAGMA table_info(cluster_deployments)").fetchall()
    print("cluster_deployments columns:")
    for c in cols:
        print(f"  {c[1]} ({c[2]})")
    
    rows = db.execute("SELECT * FROM cluster_deployments").fetchall()
    print("\nData:")
    for r in rows:
        print(r)
db.close()
