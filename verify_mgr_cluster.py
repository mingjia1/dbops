import sqlite3
db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
with db:
    # Verify instances
    rows = db.execute("SELECT name, cluster_id, host_id FROM instances WHERE name LIKE 'MGR%'").fetchall()
    print("=== Instances ===")
    for r in rows:
        print(f"  {r[0]}: cluster_id={r[1]}, host_id={r[2]}")

    # Verify statuses
    rows = db.execute("""
        SELECT i.name, st.run_status, st.health_status, st.role, st.replication_status
        FROM instance_statuses st
        JOIN instances i ON st.instance_id = i.id
        WHERE i.name LIKE 'MGR%'
    """).fetchall()
    print("\n=== Statuses ===")
    for r in rows:
        print(f"  {r[0]}: run={r[1]}, health={r[2]}, role={r[3]}, repl={r[4]}")

    # Verify topologies
    rows = db.execute("""
        SELECT i.name, tp.cluster_id, tp.master_id, tp.slave_ids, tp.replication_mode
        FROM instance_topologies tp
        JOIN instances i ON tp.instance_id = i.id
        WHERE i.name LIKE 'MGR%'
    """).fetchall()
    print("\n=== Topologies ===")
    for r in rows:
        print(f"  {r[0]}: cluster={r[1]}, master={r[2][:8] if r[2] else None}, slaves={r[3]}, mode={r[4]}")

    # Verify connection basedir
    rows = db.execute("""
        SELECT i.name, c.basedir, c.datadir
        FROM instance_connections c
        JOIN instances i ON c.instance_id = i.id
        WHERE i.name LIKE 'MGR%'
    """).fetchall()
    print("\n=== Connections ===")
    for r in rows:
        print(f"  {r[0]}: basedir={r[1]}, datadir={r[2]}")

    # Check deployment record
    d = db.execute("SELECT id, name, status, cluster_type FROM cluster_deployments WHERE id='mgr-3node-prod'").fetchone()
    print(f"\n=== Deployment ===")
    print(f"  id={d[0]}, name={d[1]}, status={d[2]}, type={d[3]}")

db.close()
