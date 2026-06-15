import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

# Try different queries on instances
c.execute('SELECT count(*) FROM instances')
print('count:', c.fetchone()[0])

c.execute('SELECT id, name FROM instances')
for r in c.fetchall():
    print('  row:', r)

c.execute('SELECT * FROM instances LIMIT 5')
cols = [d[1] for d in c.description]
print('cols:', cols)
for r in c.fetchall():
    print('  row:', r)
db.close()
