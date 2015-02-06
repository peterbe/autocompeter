import json
import hashlib

import click
import requests
import redis


_base = 'http://localhost:3000'
key = hashlib.md5(open(__file__).read()).hexdigest()

def get_blogposts():
    items = json.load(open('blogposts.json'))['items']
    for i, item in enumerate(items):
        url = 'http://www.peterbe.com/plog/%s' % item['slug']
        yield (item['title'], url, len(items) - i)


def populate(database, domain, flush=False):
    c = redis.StrictRedis(host='localhost', port=6379, db=database)
    if flush:
        c.flushdb()
    c.hset('$domainkeys', key, domain)

    for title, url, popularity in get_blogposts():
        _url = _base + '/v1'
        data = {
            'title': title,
            'url': url,
            'popularity': popularity,
        }
        print (url, title, popularity)
        r = requests.post(
            _url,
            data=data,
            headers={'Auth-Key': key}
        )
        print r.status_code
        assert r.status_code == 201, r.status_code


@click.command()
@click.option('--database', '-d', default=8)
@click.option('--domain', default='autocompeter.com')
@click.option('--flush', default=False, is_flag=True)
def run(database, domain, flush=False):
    #print (database, domain, flush)
    populate(database, domain, flush=flush)

if __name__ == '__main__':
    run()
