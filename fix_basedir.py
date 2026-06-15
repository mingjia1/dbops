import sqlite3
db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
with db:
    # Fix basedir for nodes 17 and 18
    db.execute("UPDATE instance_connections SET basedir='/usr' WHERE instance_id='3414b099-c247-45f7-9895-2bc6c5f34cf0'")
    db.execute("UPDATE instance_connections SET basedir='/usr' WHERE instance_id='25f634ec-1b15-46a7-82be-3d2f5d31a0c6'")
    print("Fixed basedir for nodes 17, 18 -> /usr")

    # Also set basedir for node 16 if empty
    c16 = db.execute("SELECT basedir FROM instance_connections WHERE instance_id='d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9'").fetchone()
    if c16 and not c16[0]:
        db.execute("UPDATE instance_connections SET basedir='/usr' WHERE instance_id='d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9'")
        print("Fixed basedir for node 16 -> /usr")
    else:
        print(f"Node 16 basedir already set: {c16[0] if c16 else 'N/A'}")

db.close()
