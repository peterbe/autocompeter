#!/usr/bin/env python

import os
import json
import hashlib
import time

import click
import requests
import redis


_base = 'http://localhost:3000'
key = hashlib.md5(open(__file__).read()).hexdigest()
_here = os.path.dirname(__file__)


def get_blogposts():
    items = json.load(open(os.path.join(_here, 'blogposts.json')))['items']
    for i, item in enumerate(items):
        url = 'http://www.peterbe.com/plog/%s' % item['slug']
        yield (item['title'], url, len(items) - i, None)

def get_events():
    items = json.load(open(os.path.join(_here, 'airmoevents.json')))['items']
    for i, item in enumerate(items):
        group = item['group']
        group = group != 'public' and group or ''
        yield (item['title'], item['url'], item['popularity'], group)


def populate(database, destination, domain, flush=False, bulk=False):
    c = redis.StrictRedis(host='localhost', port=6379, db=database)
    if flush:
        c.flushdb()
    print "KEY", key
    print "DOMAIN", domain
    c.hset('$domainkeys', key, domain)

    items = get_blogposts()
    #items = get_events()
    t0 = time.time()
    if bulk:
        _in_bulk(destination, items)
    else:
        _one_at_a_time(destination, items)
    t1 = time.time()
    print "TOOK", t1 - t0


def _one_at_a_time(destination, items):
    for title, url, popularity, group in items:
        _url = destination + '/v1'
        data = {
            'title': title,
            'url': url,
            'popularity': popularity,
            'group': group,
        }
        #print (url, title, popularity, group)
        r = requests.post(
            _url,
            data=data,
            headers={'Auth-Key': key}
        )
        #print r.status_code
        assert r.status_code == 201, r.status_code


def _in_bulk(destination, items):
    data = {
        'documents': [
            dict(
                title=t,
                url=u,
                popularity=p,
                group=g
            )
            for t, u, p, g in items
        ]
    }
    _url = destination + '/v1/bulk'
    r = requests.post(
        _url,
        data=json.dumps(data),
        headers={'Auth-Key': key}
    )
    assert r.status_code == 201, r.status_code


@click.command()
@click.option('--database', '-d', default=8)
@click.option('--destination', default='https://autocompeter.com')
@click.option('--domain', default='autocompeter.com')
@click.option('--flush', default=False, is_flag=True)
@click.option('--bulk', default=False, is_flag=True)
def run(database, destination, domain, flush=False, bulk=False):
    #print (database, domain, flush)
    populate(
        database,
        destination,
        domain,
        flush=flush,
        bulk=bulk,
    )

if __name__ == '__main__':
    run()
