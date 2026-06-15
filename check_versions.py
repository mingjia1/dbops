import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

# Get all instances
c.execute('SELECT * FROM instances')
cols = [d[1] for d in c.execute('PRAGMA table_info(instances)').fetchall()]
rows = c.fetchall()
print('=== INSTANCES DATA ===')
for row in rows:
    print('---')
    for i, col in enumerate(cols):
        print(f'  {col}: {row[i]}')

# Get connections for IP info
c.execute('SELECT instance_id, host, port, username FROM instance_connections')
print('\n=== CONNECTIONS ===')
for r in c.fetchall():
    print(f'  {r[0][:8]}... -> {r[1]}:{r[2]} ({r[3]})')
db.close()
