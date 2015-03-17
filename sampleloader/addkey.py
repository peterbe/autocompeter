#!/usr/bin/env python

import random
import hashlib

import click
import redis


@click.command()
@click.argument('domain')
@click.argument('key', default='')
@click.option('--database', '-d', default=8)
@click.option('--username', '-u', default='peterbe')
def run(domain, key, database, username):
    c = redis.StrictRedis(host='localhost', port=6379, db=database)
    if not key:
        key = hashlib.md5(str(random.random())).hexdigest()
        print "Key for %s is: %s" % (domain, key)
    c.hset('$domainkeys', key, domain)
    c.sadd('$userdomains$%s' % username, key)


if __name__ == '__main__':
    run()
