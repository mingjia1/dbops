import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

c.execute('PRAGMA table_info(hosts)')
print('=== HOSTS SCHEMA ===')
cols = [r for r in c.fetchall()]
for col in cols:
    print(f'  {col}')

c.execute('SELECT * FROM hosts')
rows = c.fetchall()
print(f'\n=== HOSTS DATA ({len(rows)} rows) ===')
for row in rows:
    print('---')
    for i, colname in enumerate([c[1] for c in cols]):
        val = str(row[i]) if row[i] else 'NULL'
        if len(val) > 60:
            val = val[:60] + '...'
        print(f'  {colname}: {val}')
db.close()
