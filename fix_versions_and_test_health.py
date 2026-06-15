import sqlite3, requests, json

# 1. Populate instance_versions
db = sqlite3.connect(r'D:\test_tmple\new_dbops\dbops\platform-backend\data\dbops.db')
c = db.cursor()

versions_data = [
    ('d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9', 'MySQL', '8.0', '8.0.46-0ubuntu0.22.04.2', None, None, 1, None, None),
    ('3414b099-c247-45f7-9895-2bc6c5f34cf0', 'MySQL', '8.0', '8.0.46-0ubuntu0.22.04.2', None, None, 1, None, None),
    ('25f634ec-1b15-46a7-82be-3d2f5d31a0c6', 'MySQL', '8.0', '8.0.46-0ubuntu0.22.04.2', None, None, 1, None, None),
]

import uuid
for iid, flavor, ver, full, rel, eol, lts, feat, eng in versions_data:
    vid = str(uuid.uuid4())
    c.execute('''INSERT INTO instance_versions (id, instance_id, flavor, version, full_version, release_date, eol_date, is_lts, features, engines)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)''',
              (vid, iid, flavor, ver, full, rel, eol, lts, feat, eng))
    print(f'Inserted version for {iid[:8]}... id={vid[:8]}...')

db.commit()
db.close()
print('\nInstance versions populated!')

# 2. Also verify health check API now works
r = requests.post('http://localhost:8080/api/v1/auth/login',
                  json={'username': 'codexadmin131813', 'password': 'admin123'}, timeout=10)
tok = r.json()['data']['token']

for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9',
            '3414b099-c247-45f7-9895-2bc6c5f34cf0',
            '25f634ec-1b15-46a7-82be-3d2f5d31a0c6']:
    r2 = requests.post(f'http://localhost:8080/api/v1/instances/{iid}/health-check',
                       headers={'Authorization': f'Bearer {tok}'}, timeout=15)
    data = r2.json()
    print(f'Health {iid[:8]}...: code={data.get("code")} msg={data.get("message")}')
    if data.get('data'):
        d = data['data']
        print(f'  status={d.get("status")} detail={str(d.get("detail",""))[:100]}')
