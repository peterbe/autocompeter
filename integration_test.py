import redis
import requests

c = redis.StrictRedis(host='localhost', port=6379, db=7)
c.flushall()


r= requests.post('http://localhost:3000/v1', {
    'domain': 'peterbecom',
    'url': ' /plog/something   ',
    # 'url': '  ',
    'popularity': "1.2.x",
    'title': "This is a blog about something",
    "groups": "private,public",
})
assert r.status_code == 400, r.status_code


r= requests.post('http://localhost:3000/v1', {
    'domain': 'peterbecom',
    'url': ' /plog/something   ',
    # 'url': '  ',
    'popularity': "1.2",
    'title': "This is a blog about something",
    "groups": "private,public",
})
assert r.status_code == 201, r.status_code
for k in r.headers:
    print "\t%s=%r" %(k, r.headers[k])
print r.content
print "--------"
r= requests.post('http://localhost:3000/v1', {
    'domain': 'peterbecom',
    'url': ' /plog/else  ',
    # 'url': '  ',
    'popularity': "1.1",
    'title': "Blogged about something else",
    "groups": "private,public",
})
assert r.status_code == 201, r.status_code
for k in r.headers:
    print "\t%s=%r" %(k, r.headers[k])
print r.content
print "--------"
r= requests.get('http://localhost:3000/v1?q=blo&domain=peterbecom')
assert r.status_code == 200, r.status_code
# for k in r.headers:
#     print "\t%s=%r" %(k, r.headers[k])
print r.content
