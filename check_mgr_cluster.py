import sqlite3, datetime, json

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')

with db:
    # Check tables
    tables = db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%instance%'").fetchall()
    print("Instance-related tables:", [t[0] for t in tables])

    # Check current instance data
    instances = db.execute("SELECT id, name, cluster_id, host_id FROM instances WHERE name LIKE 'MGR%'").fetchall()
    print("\nCurrent instances:")
    for inst in instances:
        print(f"  id={inst[0]}, name={inst[1]}, cluster_id={inst[2]}, host_id={inst[3]}")

    # Check table columns
    for tbl in ['instance_statuses', 'instance_topologies', 'instance_connections']:
        cols = db.execute(f"PRAGMA table_info({tbl})").fetchall()
        print(f"\n{tbl} columns:")
        for c in cols:
            print(f"  {c[1]} ({c[2]})")

    # Get connections for MGR instances
    print("\nInstance connections:")
    conns = db.execute("""
        SELECT ic.instance_id, ic.host, ic.port 
        FROM instance_connections ic 
        JOIN instances i ON ic.instance_id = i.id 
        WHERE i.name LIKE 'MGR%'
    """).fetchall()
    for c in conns:
        print(f"  instance_id={c[0]}, host={c[1]}, port={c[2]}")

    # Get status and topology
    print("\nInstance statuses:")
    statuses = db.execute("""
        SELECT instance_id, run_status, health_status, role, replication_status 
        FROM instance_statuses WHERE instance_id IN (
            SELECT id FROM instances WHERE name LIKE 'MGR%'
        )
    """).fetchall()
    for s in statuses:
        print(f"  instance_id={s[0]}, run={s[1]}, health={s[2]}, role={s[3]}, repl={s[4]}")

    print("\nInstance topologies:")
    tops = db.execute("""
        SELECT instance_id, cluster_id, master_id, slave_ids, replication_mode
        FROM instance_topologies WHERE instance_id IN (
            SELECT id FROM instances WHERE name LIKE 'MGR%'
        )
    """).fetchall()
    for t in tops:
        print(f"  instance_id={t[0]}, cluster_id={t[1]}, master={t[2]}, slaves={t[3]}, mode={t[4]}")

db.close()
