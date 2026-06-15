import requests

r = requests.post('http://localhost:8080/api/v1/auth/login',
                  json={'username': 'codexadmin131813', 'password': 'admin123'}, timeout=10)
tok = r.json()['data']['token']
headers = {'Authorization': f'Bearer {tok}'}

# Check-tools raw response
print('=== Check Tools (raw) ===')
for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9']:
    r4 = requests.get(f'http://localhost:8080/api/v1/instances/{iid}/check-tools', headers=headers, timeout=15)
    print(f'Status: {r4.status_code}')
    print(f'Headers: {dict(r4.headers)}')
    print(f'Body: {r4.text[:300]}')

# Env check raw
print('\n=== Env Check (raw) ===')
r2 = requests.post('http://localhost:8080/api/v1/env-checks', json={'instance_ids': ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9','3414b099-c247-45f7-9895-2bc6c5f34cf0','25f634ec-1b15-46a7-82be-3d2f5d31a0c6']}, headers=headers, timeout=15)
print(f'Status: {r2.status_code}')
print(f'Body: {r2.text[:500]}')

# Check if agent-status route exists
r5 = requests.get(f'http://localhost:8080/api/v1/instances/d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9/agent-status', headers=headers, timeout=15)
print(f'\nAgent status: {r5.status_code} {r5.text[:200]}')
