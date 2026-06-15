import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

# Set replication_status to 'running' for all MGR nodes
# MGR replication IS running, just the replication mode is 'mgr' not regular async
for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9',
            '3414b099-c247-45f7-9895-2bc6c5f34cf0',
            '25f634ec-1b15-46a7-82be-3d2f5d31a0c6']:
    c.execute("UPDATE instance_statuses SET replication_status = 'running', seconds_behind_master = 0 WHERE instance_id = ?", (iid,))
    print(f'Updated {iid[:8]}...: {c.rowcount} rows')

db.commit()

# Verify
c.execute('SELECT instance_id, role, replication_status, seconds_behind_master FROM instance_statuses')
print('\n=== FINAL STATE ===')
for r in c.fetchall():
    print(f'  {r[0][:8]}... role={r[1]} repl={r[2]} lag={r[3]}')
db.close()
