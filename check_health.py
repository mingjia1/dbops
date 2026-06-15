import requests, json

r = requests.post('http://localhost:8080/api/v1/auth/login',
                  json={'username': 'codexadmin131813', 'password': 'admin123'}, timeout=10)
tok = r.json()['data']['token']

iid = 'd6abd9f2-e47e-4dbd-b558-7cc841a3f9c9'
r2 = requests.post(f'http://localhost:8080/api/v1/instances/{iid}/health-check',
                   headers={'Authorization': f'Bearer {tok}'}, timeout=15)
print(r2.status_code, json.dumps(r2.json(), indent=2, ensure_ascii=False)[:500])
