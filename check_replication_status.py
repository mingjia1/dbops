import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

c.execute('SELECT instance_id, run_status, health_status, role, replication_status, seconds_behind_master FROM instance_statuses')
cols = [d[1] for d in c.description]
for row in c.fetchall():
    print('---')
    for i, col in enumerate(cols):
        print(f'  {col}: {row[i]}')
db.close()
