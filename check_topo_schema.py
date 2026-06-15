import sqlite3
db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
with db:
    tops = db.execute('SELECT instance_id, cluster_id, master_id FROM instance_topologies').fetchall()
    print('All topologies:', tops)
    idx = db.execute("SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name='instance_topologies'").fetchall()
    for i in idx:
        print('Index:', i[0], '->', i[1])
    tbl = db.execute("SELECT sql FROM sqlite_master WHERE type='table' AND name='instance_topologies'").fetchone()
    print('Table SQL:', tbl[0])
db.close()
