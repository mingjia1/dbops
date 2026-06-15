import requests, json

users = [
    ('codexadmin131813', 'admin123'),
    ('admin', 'admin123'),
]
for u, p in users:
    r = requests.post('http://localhost:8080/api/v1/auth/login',
                      json={'username': u, 'password': p}, timeout=10)
    data = r.json()
    print(f'{u}/{p}: status={r.status_code} code={data.get("code")} msg={data.get("message")}')
