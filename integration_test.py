# -*- coding: utf-8 -*-

import redis
import requests

c = redis.StrictRedis(host='localhost', port=6379, db=7)
c.flushdb()

r= requests.get('http://localhost:3000/')
assert r.status_code == 200, r.status_code


r= requests.post('http://localhost:3000/v1', {
    'domain': 'peterbecom',
    'url': ' /plog/something   ',
    # 'url': '  ',
    'popularity': "1.2.x",
    'title': "This is a blog about something",
    "groups": "private,public",
})
assert r.status_code == 422, r.status_code
r= requests.post('http://localhost:3000/v1', {
    'domain': ' ',
    'url': ' /plog/something   ',
    # 'url': '  ',
    'popularity': "11",
    'title': "This is a blog about something",
    "groups": "private,public",
})
assert r.status_code == 422, r.status_code


r= requests.post('http://localhost:3000/v1', {
    'domain': 'peterbecom',
    'url': ' /plog/something   ',
    # 'url': '  ',
    'popularity': "12",
    'title': "This is a blog about something",
    "groups": "private,public",
})
assert r.status_code == 201, r.status_code
r= requests.post('http://localhost:3000/v1', {
    'domain': 'air.mozilla.org    ',
    'url': ' /monday-meeting',
    # 'url': '  ',
    'popularity': "0",
    'title': "Peter talks about blogging",
    "groups": "private,public",
})
assert r.status_code == 201, r.status_code
# for k in r.headers:
#     print "\t%s=%r" %(k, r.headers[k])
# print r.content
# print "--------"
r= requests.post('http://localhost:3000/v1', {
    'domain': 'peterbecom',
    'url': ' /plog/else  ',
    # 'url': '  ',
    'popularity': "11",
    'title': u"Blogged aböut something else",
    "groups": "private,public",
})
assert r.status_code == 201, r.status_code
# for k in r.headers:
#     print "\t%s=%r" %(k, r.headers[k])
# print r.content
# print "--------"
# not a valid number
r= requests.get('http://localhost:3000/v1?q=blo&domain=peterbecom&n=x')
# see https://github.com/mholt/binding/issues/31
assert r.status_code == 422, r.status_code

# no domain
r= requests.get('http://localhost:3000/v1?q=blo')
# see https://github.com/mholt/binding/issues/31
assert r.status_code == 422, r.status_code

r= requests.get('http://localhost:3000/v1?q=blo&domain=peterbecom%20')
assert r.status_code == 200, r.status_code
# for k in r.headers:
#     print "\t%s=%r" %(k, r.headers[k])
print r.json()
# print r.json()['results'][1][1]

r= requests.get('http://localhost:3000/v1?q=blo&domain=peterbecom&n=1')
assert r.status_code == 200, r.status_code
assert len(r.json()['results']) == 1, r.json()['results']


r= requests.get('http://localhost:3000/v1?q=blo&domain=air.mozilla.org')
assert r.status_code == 200, r.status_code
print r.json()
