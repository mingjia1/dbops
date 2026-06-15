import bcrypt
import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()
c.execute('SELECT id, username, password, role, status FROM users WHERE username = ?', ('admin',))
row = c.fetchone()
db.close()

if row:
    uid, uname, pwhash, role, status = row
    print(f'ID: {uid}')
    print(f'Username: {uname}')
    print(f'Hash: {pwhash}')
    print(f'Role: {role}')
    print(f'Status: {status}')
    
    for pw in ['admin123', 'Hcfc@DboOps#2024_80', 'codex123', 'password123']:
        try:
            result = bcrypt.checkpw(pw.encode(), pwhash.encode() if isinstance(pwhash, str) else pwhash)
            print(f'  Password "{pw}": {result}')
        except Exception as e:
            print(f'  Password "{pw}": ERROR {e}')
else:
    print('Admin user not found')
