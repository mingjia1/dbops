import sqlite3

db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

new_encrypted = 'JrzPMmm5XdmAyl/JiXKr0Ba/w4ABCBhnlpw/vSOsigk9894X6FJAKlYWHE1/Itw='

# Update all 3 instances
for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9',
            '3414b099-c247-45f7-9895-2bc6c5f34cf0',
            '25f634ec-1b15-46a7-82be-3d2f5d31a0c6']:
    c.execute('UPDATE instance_connections SET password_encrypted = ? WHERE instance_id = ?',
              (new_encrypted, iid))
    print(f'Updated {iid}: {c.rowcount} rows')

db.commit()
db.close()
print('Done')
