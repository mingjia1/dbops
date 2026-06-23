@echo off
python -c "import json; print(json.load(open('login.json'))['data']['token'])" > token.txt 2>/dev/null
for /f %%t in (token.txt) do set TOKEN=%%t

echo === Single status check ===
curl -s -X POST "http://127.0.0.1:8080/api/v1/hosts" -H "Content-Type: application/json" -H "Authorization: Bearer %TOKEN%" > /dev/null 2>&1

echo === GET hosts ===
curl -s "http://127.0.0.1:8080/api/v1/hosts" -H "Authorization: Bearer %TOKEN%" > hosts.json
python -c "import json; d=json.load(open('hosts.json')); hosts=d.get('data',[]); print(f'Found {len(hosts)} hosts'); [print(f'  {h[\"id\"][:8]}... {h[\"name\"]} {h[\"address\"]} status={h.get(\"status\",\"?\")}') for h in hosts[:3]]"
