import requests, json
r = requests.post('http://localhost:8080/api/v1/auth/login', json={'username':'codexadmin131813','password':'admin123'}, timeout=10)
tok = r.json()['data']['token']
r2 = requests.get('http://localhost:8080/api/v1/topology/clusters/mgr-3node-prod/graph', headers={'Authorization':f'Bearer {tok}'}, timeout=10)
data = r2.json()['data']
for n in data['nodes']:
    print("%s: role=%s status=%s" % (n['name'], n['role'], n['status']))
