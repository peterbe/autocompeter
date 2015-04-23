#!/usr/bin/env python

import os
import json
import hashlib
import time
import glob

import click
import requests
import redis


_base = 'http://localhost:3000'
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


def get_items(jsonfile):
    items = json.load(open(jsonfile))['items']
    for i, item in enumerate(items):
        if 'group' in item:
            group = item['group']
            group = group != 'public' and group or ''
        else:
            group = None
        yield (item['title'], item['url'], item.get('popularity'), group)


def populate(database, destination, domain, jsonfile, flush=False, bulk=False):
    c = redis.StrictRedis(host='localhost', port=6379, db=database)
    if flush:
        c.flushdb()
    key = hashlib.md5(open(__file__).read()).hexdigest()

    print "KEY", key
    print "DOMAIN", domain
    c.hset('$domainkeys', key, domain)
    c.sadd('$userdomains$peterbe', key)

    items = get_items(jsonfile)
    # items = get_blogposts()
    #items = get_events()
    t0 = time.time()
    if bulk:
        _in_bulk(destination, items, key)
    else:
        _one_at_a_time(destination, items, key)
    t1 = time.time()
    print "TOOK", t1 - t0


def _one_at_a_time(destination, items, key):
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


def _in_bulk(destination, items, key):
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


_json_files = glob.glob(os.path.join(_here, '*.json'))

@click.command()
@click.option('--database', '-d', default=8)
@click.option('--destination', default='https://autocompeter.com')
@click.option('--domain', default='autocompeter.com')
@click.option('--dataset', default='blogposts.json')
@click.option('--flush', default=False, is_flag=True)
@click.option('--no-bulk', default=False, is_flag=True)
def run(database, destination, domain, dataset, flush=False, no_bulk=True):
    # print (database, domain, flush)
    bulk = not no_bulk
    for filename in _json_files:
        if os.path.basename(filename) == dataset:
            jsonfile = filename
            break
    else:
        raise ValueError("dataset %r not recognized" % dataset)
    populate(
        database,
        destination,
        domain,
        jsonfile,
        flush=flush,
        bulk=bulk,
    )

if __name__ == '__main__':
    run()
