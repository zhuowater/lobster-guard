import requests, json, sys
base='http://127.0.0.1:9090'
H={'Authorization':'Bearer hermes-test-token'}
results=[]

def check(name, cond, detail):
    results.append((name, bool(cond), detail))

# L1: targeted API blackbox for sender identity display fields
u=requests.get(base+'/api/v1/users/risk-top?limit=5', headers=H, timeout=10).json()['users']
check('L1 user-profiles display_name', all(x.get('display_name') for x in u[:3]), u[:3])
check('L1 user-profiles department', all(x.get('department') for x in u[:3]), u[:3])

b=requests.get(base+'/api/v1/behavior/profiles?limit=5', headers=H, timeout=10).json()['profiles']
check('L1 behavior display_name', all(x.get('display_name') for x in b[:3]), b[:3])
check('L1 behavior department', all(x.get('department') for x in b[:3]), b[:3])

s=requests.get(base+'/api/v1/sessions/replay?limit=5', headers=H, timeout=10).json()['sessions']
check('L1 sessions display_name', all(x.get('display_name') for x in s[:3]), s[:3])
check('L1 sessions department', all(x.get('department') for x in s[:3]), s[:3])

a=requests.get(base+'/api/v1/audit/logs?limit=5', headers=H, timeout=10).json()['logs']
check('L1 audit display_name', all(x.get('display_name') for x in a[:3]), a[:3])
check('L1 audit department', all(x.get('department') for x in a[:3]), a[:3])

# L3 deploy validation: service health + demo seeded + page APIs return 200
health=requests.get(base+'/healthz', timeout=10)
check('L3 healthz 200', health.status_code==200, health.text[:200])
seed=requests.post(base+'/api/v1/demo/seed', headers=H, timeout=20)
check('L3 demo seed 200', seed.status_code==200 and seed.json().get('ok') is True, seed.text[:400])
for path in ['/api/v1/users/risk-top?limit=3','/api/v1/behavior/profiles?limit=3','/api/v1/sessions/replay?limit=3','/api/v1/audit/logs?limit=3']:
    r=requests.get(base+path, headers=H, timeout=15)
    check('L3 GET '+path, r.status_code==200, r.text[:200])

ok=True
for name, passed, detail in results:
    print(('PASS' if passed else 'FAIL'), name)
    if not passed:
        ok=False
        print(detail)
print('SUMMARY', sum(1 for _,p,_ in results if p), '/', len(results))
sys.exit(0 if ok else 1)
