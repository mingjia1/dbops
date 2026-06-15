import requests, json

r = requests.post('http://localhost:8080/api/v1/auth/login',
                  json={'username': 'codexadmin131813', 'password': 'admin123'}, timeout=10)
tok = r.json()['data']['token']
headers = {'Authorization': f'Bearer {tok}'}

# Test Agent status check
print('=== Agent Status (via host agent action) ===')
for hid in ['61168e37-9022-43e2-8722-7100218ad8b3',
            '6e45b3fa-1eda-4cb7-b16c-ec97c928e680',
            '26b1b5f9-e751-447c-94f2-50d214aa1591']:
    r2 = requests.post(f'http://localhost:8080/api/v1/hosts/{hid}/agent',
                       json={'action': 'status', 'agent_port': 9090},
                       headers=headers, timeout=15)
    print(f'  {hid[:8]}...: status={r2.status_code} body={json.dumps(r2.json(), ensure_ascii=False)[:200]}')

# Test batch health check (一键检测 via instance health-check)
print('\n=== Batch Health Check ===')
for iid in ['d6abd9f2-e47e-4dbd-b558-7cc841a3f9c9',
            '3414b099-c247-45f7-9895-2bc6c5f34cf0',
            '25f634ec-1b15-46a7-82be-3d2f5d31a0c6']:
    r3 = requests.post(f'http://localhost:8080/api/v1/instances/{iid}/health-check',
                       headers=headers, timeout=15)
    print(f'  {iid[:8]}...: status={r3.status_code} msg={r3.json().get("message","?")} data_status={r3.json().get("data",{}).get("status","?")}')
