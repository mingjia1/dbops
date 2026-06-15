import sqlite3
db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
with db:
    rows = db.execute("""
        SELECT i.name, ic.username, ic.password_encrypted
        FROM instance_connections ic
        JOIN instances i ON ic.instance_id = i.id
        WHERE i.name LIKE 'MGR%'
    """).fetchall()
    for r in rows:
        print(f"{r[0]}: user={r[1]}, pass_encrypted={r[2]}")

    # Also check deploy requests / tasks for any password hints
    rows = db.execute("SELECT id, request FROM tasks WHERE instance_id IN (SELECT id FROM instances WHERE name LIKE 'MGR%') ORDER BY created_at DESC LIMIT 5").fetchall()
    for r in rows:
        print(f"\nTask {r[0]}: {r[1][:200] if r[1] else 'None'}")
db.close()
