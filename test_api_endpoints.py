import requests, json

r = requests.post('http://localhost:8080/api/v1/auth/login',
                  json={'username': 'codexadmin131813', 'password': 'admin123'}, timeout=10)
tok = r.json()['data']['token']
headers = {'Authorization': f'Bearer {tok}'}

# Test env check (one-click detection)
print('=== Env Check ===')
r2 = requests.post('http://localhost:8080/api/v1/env-checks', json={'instance_ids': ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9','3414b099-c247-45f7-9895-2bc6c5f34cf0','25f634ec-1b15-46a7-82be-3d2f5d31a0c6']}, headers=headers, timeout=15)
print(f'  status={r2.status_code} body={json.dumps(r2.json(), ensure_ascii=False)[:200]}')

# Test backup scan
print('\n=== Backup Scan ===')
r3 = requests.post('http://localhost:8080/api/v1/backups/scan', json={'instance_id': 'd6abd9f2-e47e-4dbd-b558-7cc841a3f9c9', 'backup_dir': '/data/backup'}, headers=headers, timeout=15)
print(f'  status={r3.status_code} body={json.dumps(r3.json(), ensure_ascii=False)[:200]}')

# Test check-tools on instance
print('\n=== Check Tools ===')
for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9']:
    r4 = requests.get(f'http://localhost:8080/api/v1/instances/{iid}/check-tools', headers=headers, timeout=15)
    print(f'  {iid[:8]}...: status={r4.status_code} body={json.dumps(r4.json(), ensure_ascii=False)[:200]}')

# Agent status check
print('\n=== Agent Status ===')
for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9','3414b099-c247-45f7-9895-2bc6c5f34cf0','25f634ec-1b15-46a7-82be-3d2f5d31a0c6']:
    r5 = requests.get(f'http://localhost:8080/api/v1/instances/{iid}/agent-status', headers=headers, timeout=15)
    print(f'  {iid[:8]}...: status={r5.status_code} body={json.dumps(r5.json(), ensure_ascii=False)[:200]}')
