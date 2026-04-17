import json
import time
import urllib.request
import os

BASE='http://localhost:8000'
API_KEY='fs_dev_engineer_ci-key_6099171d2c319b9951ff102386b073bc74896c26a037b1b213caafea7f54a4e7'
OUTDIR='d:/FailSafe/deployments/docker/experiments/results/quick_runs'
os.makedirs(OUTDIR, exist_ok=True)

services = {
    'order-service': ['http://localhost:8082/orders/10'],
    'user-service': ['http://localhost:8081/users/1'],
    'payment-service': ['http://localhost:8083/payments/10'],
}

faults = [
    {'fault_type':'cpu_stress','duration':30,'max_intensity':40},
    {'fault_type':'memory_stress','duration':30,'max_intensity':40},
    {'fault_type':'network_delay','duration':30,'max_intensity':40},
]

headers={'Content-Type':'application/json','x-api-key':API_KEY}

results=[]

def post_start(payload):
    req=urllib.request.Request(BASE+'/experiments/backend/start', data=json.dumps(payload).encode(), headers=headers)
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read().decode())

def get_status(expid):
    req=urllib.request.Request(BASE+f'/experiments/backend/status?id={expid}', headers={})
    with urllib.request.urlopen(req, timeout=10) as r:
        return json.loads(r.read().decode())

def get_metrics(expid):
    req=urllib.request.Request(BASE+f'/experiments/backend/metrics?id={expid}', headers={'x-api-key':API_KEY})
    with urllib.request.urlopen(req, timeout=10) as r:
        return json.loads(r.read().decode())

for svc, endpoints in services.items():
    for f in faults:
        payload={
            'target_type':'docker',
            'fault_type': f['fault_type'],
            'duration': f['duration'],
            'targets':[svc],
            'observed_endpoints': endpoints,
            'observation_type':'http',
            'max_intensity': f['max_intensity'],
        }
        try:
            print('Starting', svc, f['fault_type'])
            resp = post_start(payload)
            expid = resp.get('id')
            if not expid:
                print('no id in response', resp)
                results.append((svc,f['fault_type'],'start_failed',resp))
                continue
            # poll
            for i in range(60):
                try:
                    st = get_status(expid)
                    state = st.get('experiment',{}).get('state')
                    phase = st.get('experiment',{}).get('phase')
                    print(expid, state, phase)
                    if state in ('completed','failed'):
                        break
                except Exception as e:
                    print('status err',e)
                time.sleep(2)
            # fetch metrics
            try:
                metrics = get_metrics(expid)
            except Exception as e:
                metrics = {'error': str(e)}
            # save
            prefix = os.path.join(OUTDIR, expid)
            with open(prefix+'-status.json','w',encoding='utf-8') as fh:
                json.dump(st, fh, indent=2, ensure_ascii=False)
            with open(prefix+'-metrics.json','w',encoding='utf-8') as fh:
                json.dump(metrics, fh, indent=2, ensure_ascii=False)
            results.append((svc,f['fault_type'],'ok',expid))
        except Exception as e:
            print('failed',svc,f['fault_type'],e)
            results.append((svc,f['fault_type'],'error',str(e)))

print('\nSummary:')
for r in results:
    print(r)
