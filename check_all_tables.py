import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

# List all tables
c.execute("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
tables = [r[0] for r in c.fetchall()]
print('=== ALL TABLES ===')
for t in tables:
    c.execute(f'SELECT COUNT(*) FROM "{t}"')
    cnt = c.fetchone()[0]
    print(f'  {t}: {cnt} rows')
db.close()
