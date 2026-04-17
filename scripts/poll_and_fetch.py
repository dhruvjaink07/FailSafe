import time, urllib.request, json
id='a1cdc7e0-1f85-4e30-a9d5-255630cf806a'
url='http://localhost:8000/experiments/backend/status?id=' + id
for i in range(40):
    try:
        r=urllib.request.urlopen(url, timeout=5)
        s=json.loads(r.read().decode())
        state=s.get('experiment',{}).get('state')
        phase=s.get('experiment',{}).get('phase')
        print(i, 'state',state,'phase',phase)
        if state in ('completed','failed'):
            break
    except Exception as e:
        print('err',e)
    time.sleep(2)
# fetch metrics
murl='http://localhost:8000/experiments/backend/metrics?id='+id
try:
    req=urllib.request.Request(murl, headers={'x-api-key':'fs_dev_engineer_ci-key_6099171d2c319b9951ff102386b073bc74896c26a037b1b213caafea7f54a4e7'})
    r=urllib.request.urlopen(req, timeout=10)
    m=json.loads(r.read().decode())
    eps=m.get('endpoints',{})
    print('endpoints count',len(eps))
    for k,v in list(eps.items())[:5]:
        print(k, 'degraded=', v.get('degraded'), 'stability=', v.get('stability_score'), 'errors=', v.get('errors'))
except Exception as e:
    print('metrics err',e)
