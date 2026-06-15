import sqlite3, bcrypt

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()
c.execute('SELECT id, username, password, email, role, status, created_at, updated_at FROM users WHERE username = ?', ('admin',))
row = c.fetchone()
db.close()

if row:
    print('ID:', repr(row[0]))
    print('Username:', repr(row[1]))
    print('Password:', repr(row[2][:40]))
    print('Email:', repr(row[3]))
    print('Role:', repr(row[4]))
    print('Status:', repr(row[5]))
    print('Created:', repr(row[6]))
    
    verified = bcrypt.checkpw(b'admin123', row[2].encode() if isinstance(row[2], str) else row[2])
    print('admin123 matches:', verified)
else:
    print('admin user not found')
