# -*- coding: utf-8 -*-
"""
End-to-end tests that flush the Redis database (number 8).
These tests expect to be able to talk to the running server on
localhost:8000.
"""

import datetime
import json
import unittest

from nose.tools import ok_, eq_
import redis
import requests


class E2E(unittest.TestCase):

    _base = 'http://localhost:3000'

    @classmethod
    def setUpClass(cls):
        cls.c = redis.StrictRedis(host='localhost', port=6379, db=8)

    def setUp(self):
        self.c.flushdb()
        assert self.c.dbsize() == 0, self.c.dbsize()
        self._set_domain_key('xyz123', 'peterbecom')

    def _set_domain_key(self, key, domain):
        self.c.hset('$domainkeys', key, domain)

    def get(self, url, *args, **kwargs):
        return requests.get(self._base + url, *args, **kwargs)

    def post(self, url, *args, **kwargs):
        return requests.post(self._base + url, *args, **kwargs)

    def delete(self, url, *args, **kwargs):
        return requests.delete(self._base + url, *args, **kwargs)

    def test_homepage(self):
        r = self.get('/')
        eq_(r.status_code, 200)

    def test_ping(self):
        r = self.get('/v1/ping')
        eq_(r.status_code, 200)
        eq_(r.content, 'pong\n')
        eq_(r.headers['Access-Control-Allow-Origin'], '*')

    def test_404(self):
        r = self.get('/gobblygook')
        eq_(r.status_code, 404)

    def test_post_bad_number(self):
        r = self.post('/v1', {
            'url': ' /plog/something   ',
            'popularity': "1.2.x",
            'title': "This is a blog about something",
            "group": "public",
        }, headers={'Auth-Key': 'xyz123'})
        ok_(r.status_code >= 400 and r.status_code < 500)

    def test_bad_key(self):
        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
        }, headers={})  # not set at all
        eq_(r.status_code, 403)

        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
        }, headers={'Auth-Key': ''})  # empty
        eq_(r.status_code, 403)

        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'junkjunk'})  # junk
        eq_(r.status_code, 403)

    def test_post_ok(self):
        r = self.post('/v1', {
            'url': ' /plog/something   ',
            'popularity': "12",
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=blo&d=peterbecom')
        eq_(
            r.json(),
            {
                'terms': [u'blog'],
                'results': [
                    [
                        u'/plog/something',
                        u'This is a blog about something'
                    ]
                ]
            }
        )
        eq_(r.headers['Access-Control-Allow-Origin'], '*')

    def test_search_on_numbers(self):
        r = self.post('/v1', {
            'url': '/plog/2015',
            'title': "Monday Meeting 2015",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.post('/v1', {
            'url': '/plog/2014',
            'title': "Monday Meeting 2014",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=monday&d=peterbecom')
        terms = r.json()['terms']
        eq_(terms, ['monday'])
        results = r.json()['results']
        eq_(len(results), 2)

        r = self.get('/v1?q=monday%202015&d=peterbecom')
        terms = r.json()['terms']
        eq_(terms, ['monday', '2015'])
        results = r.json()['results']
        print r.json()
        eq_(len(results), 1)

    def test_search_with_spellcorrection(self):
        r = self.post('/v1', {
            'url': '/plog/2015',
            'title': "Monday Meeting 2015",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=monda&d=peterbecom')
        terms = r.json()['terms']
        eq_(terms, ['monday'])
        results = r.json()['results']
        eq_(len(results), 1)

        r = self.get('/v1?q=munda&d=peterbecom')
        terms = r.json()['terms']
        eq_(terms, ['monday'])
        results = r.json()['results']
        eq_(len(results), 1)

    def test_search_too_short(self):
        r = self.get('/v1?q=,&d=peterbecom')
        eq_(
            r.json(),
            {'terms': [], 'results': []}
        )

    def test_different_domains(self):
        r = self.post('/v1', {
            'url': ' /plog/something   ',
            'popularity': "12",
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        # need a new auth key for this different domain
        self._set_domain_key('abc987', 'air.mozilla.org')
        r = self.post('/v1', {
            'url': ' /some/page',
            'title': "Also about the word blog",
        }, headers={'Auth-Key': 'abc987'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=blo&d=peterbecom')
        eq_(
            r.json(),
            {
                'terms': [u'blog'],
                'results': [
                    [
                        u'/plog/something',
                        u'This is a blog about something'
                    ]
                ]
            }
        )

        r = self.get('/v1?q=blo&d=air.mozilla.org')
        eq_(
            r.json(),
            {
                'terms': [u'blog'],
                'results': [
                    [
                        u'/some/page',
                        u'Also about the word blog'
                    ]
                ]
            }
        )

    def test_unidecode(self):
        r = self.post('/v1', {
            'url': ' /some/page',
            'title': u"Blögged about something else",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=blog&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json(),
            {
                'terms': [u'blogged'],
                'results': [
                    [
                        u'/some/page',
                        u'Blögged about something else'
                    ]
                ]
            }
        )

        r = self.get(u'/v1?q=bl\xf6g&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json(),
            {
                'terms': [u'blögged', u'blogged'],
                'results': [
                    [
                        u'/some/page',
                        u'Blögged about something else'
                    ]
                ]
            }
        )

    def test_search_with_apostrophes(self):
        r = self.post('/v1', {
            'url': '/some/page',
            'title': u"The word 'watch'",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=watc&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json(),
            {
                'terms': ['watch'],
                'results': [
                    [
                        u'/some/page',
                        u"The word 'watch'"
                    ]
                ]
            }
        )

        r = self.post('/v1', {
            'url': '/some/page2',
            'title': u"At 1 o'clock there's a game",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1?q=o&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json(),
            {
                'terms': ["o'clock"],
                'results': [
                    [
                        u'/some/page2',
                        u"At 1 o'clock there's a game"
                    ]
                ]
            }
        )
        r = self.get("/v1?q=o'&d=peterbecom")
        eq_(r.status_code, 200)
        eq_(
            r.json(),
            {
                'terms': ["o'clock"],
                'results': [
                    [
                        u'/some/page2',
                        u"At 1 o'clock there's a game"
                    ]
                ]
            }
        )
        r = self.get("/v1?q=o'c&d=peterbecom")
        eq_(r.status_code, 200)
        eq_(
            r.json(),
            {
                'terms': ["o'clock"],
                'results': [
                    [
                        u'/some/page2',
                        u"At 1 o'clock there's a game"
                    ]
                ]
            }
        )

    def test_fetch_with_dfferent_n(self):
        self._set_domain_key('xyz123', 'peterbecom')
        for i in range(1, 20):
            r = self.post('/v1', {
                'url': '/%d' % i,
                'popularity': i,
                'title': u"Page %d" % i,
            }, headers={'Auth-Key': 'xyz123'})
            eq_(r.status_code, 201)

        r = self.get('/v1?q=pag&d=peterbecom')
        eq_(len(r.json()['results']), 10)

        r = self.get('/v1?q=pag&d=peterbecom&n=2')
        eq_(len(r.json()['results']), 2)

        r = self.get('/v1?q=pag&d=peterbecom&n=0')
        eq_(len(r.json()['results']), 10)

        r = self.get('/v1?q=pag&d=peterbecom&n=-1')
        eq_(len(r.json()['results']), 10)

        r = self.get('/v1?q=pag&d=peterbecom&n=x')
        ok_(r.status_code >= 400 and r.status_code < 500)

    def test_fetch_without_domain(self):
        r = self.get('/v1?q=pag')
        ok_(r.status_code >= 400 and r.status_code < 500)

    def test_sorted_by_popularity(self):
        self._set_domain_key('xyz123', 'peterbecom')
        r = self.post('/v1', {
            'url': '/minor',
            'popularity': "1.1",
            'title': u"Page Minor",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.post('/v1', {
            'url': '/major',
            'popularity': "2.7",
            'title': u"Page Major",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=pag&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json()['results'],
            [[u'/major', u'Page Major'], [u'/minor', u'Page Minor']]
        )

        # insert the Minor one again but this time with a high popularity
        r = self.post('/v1', {
            'url': '/minor',
            'popularity': "3.0",
            'title': u"Page Minor",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1?q=pag&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json()['results'],
            [[u'/minor', u'Page Minor'], [u'/major', u'Page Major']]
        )

    def test_match_multiple_words(self):

        r = self.post('/v1', {
            'url': ' /plog/something   ',
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=blog%20ab&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(
            r.json()['terms'],
            ['blog', 'about']
        )
        eq_(
            r.json()['results'],
            [[u'/plog/something', u'This is a blog about something']]
        )

    def test_clean_junk(self):
        r = self.get('/v1', params={
            'q': '[{(";.!peter?-.")}]',
            'd': 'peterbecom'
        })
        eq_(r.status_code, 200)
        eq_(
            r.json()['terms'],
            []
        )
        eq_(r.json()['results'], [])

    def test_delete_bad_auth_key(self):
        # not even set
        r = self.delete('/v1', params={
            'url': ' /plog/something   ',
        })
        eq_(r.status_code, 403)

        # set but not recognized
        r = self.delete('/v1', params={
            'url': ' /plog/something   ',
        }, headers={'Auth-Key': 'junkjunkjunk'})
        eq_(r.status_code, 403)

    def test_delete_row(self):
        self._set_domain_key('xyz123', 'peterbecom')
        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1?q=ab&d=peterbecom')
        eq_(r.status_code, 200)
        ok_(r.json()['results'])

        r = self.delete('/v1', params={
            'url': ' /plog/something   ',
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 204)
        r = self.get('/v1?q=ab&d=peterbecom')
        eq_(r.status_code, 200)
        ok_(not r.json()['results'])

    def test_delete_row_belonging_a_group(self):
        self._set_domain_key('xyz123', 'peterbecom')
        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
            'group': 'private'
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1?q=ab&d=peterbecom')
        eq_(r.status_code, 200)
        ok_(not r.json()['results'])
        r = self.get('/v1?q=ab&d=peterbecom&g=private')
        eq_(r.status_code, 200)
        ok_(r.json()['results'])

        r = self.delete('/v1', params={
            'url': ' /plog/something   ',
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 204)
        r = self.get('/v1?q=ab&d=peterbecom')
        eq_(r.status_code, 200)
        ok_(not r.json()['results'])
        r = self.get('/v1?q=ab&d=peterbecom&g=private')
        eq_(r.status_code, 200)
        ok_(not r.json()['results'])

    def test_delete_invalid_url(self):
        self._set_domain_key('xyz123', 'peterbecom')
        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.delete('/v1', params={
            'url': 'neverheardof',
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 404)

    def test_delete_row_carefully(self):
        """deleting one item, by URL, shouldn't affect other entries"""
        self._set_domain_key('xyz123', 'peterbecom')
        # first one
        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        # second one
        r = self.post('/v1', {
            'url': '/other/url',
            'title': "Another blog post about nothing",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1?q=ab&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 2)

        r = self.delete('/v1', params={
            'url': ' /plog/something   ',
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 204)
        r = self.get('/v1?q=ab&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 1)
        eq_(r.json()['results'], [
            [u'/other/url', u'Another blog post about nothing']
        ])

    def test_delete_domain(self):
        self._set_domain_key('xyz123', 'peterbecom')
        r = self.post('/v1', {
            'url': '/plog/something',
            'title': "blog something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.post('/v1', {
            'url': '/plog/other',
            'title': "Another blog",
            'group': 'private'
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 2)

        r = self.get('/v1?q=blog&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 1)
        r = self.get('/v1?q=blog&d=peterbecom&g=private')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 2)
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        ok_(stats['fetches'].values()[0])

        r = self.delete('/v1/flush')
        eq_(r.status_code, 403)
        r = self.delete('/v1/flush', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 204)
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 0)
        ok_(stats['fetches'].values()[0])

        r = self.get('/v1?q=blo&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 0)

        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 0)

    def test_search_with_groups(self):
        r = self.post('/v1', {
            'url': '/page/public',
            'popularity': 10,
            'title': 'This is a PUBLIC sample',
            'group': '',
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.post('/v1', {
            'url': '/page/private',
            'popularity': 20,
            'title': 'This is a PRIVATE page',
            'group': 'private'
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.post('/v1', {
            'url': '/page/semi',
            'popularity': 20,
            'title': 'This is a SEMI private sample',
            'group': 'semi-private'
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=thi&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 1)

        r = self.get('/v1?q=thi&d=peterbecom&g=private')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 2)

        r = self.get('/v1?q=samp&d=peterbecom&g=semi-private')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 2)

        r = self.get('/v1?q=thi&d=peterbecom&g=private,semi-private')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 3)

    def test_search_with_whole_words(self):
        """if you search for 'four thi' it should find 'Four things'
        and 'this is four items'.
        But should it really find 'fourier thinking'?
        """
        r = self.post('/v1', {
            'url': '/page/first',
            'popularity': 1,
            'title': 'Four special things',
        }, headers={'Auth-Key': 'xyz123'})
        r = self.post('/v1', {
            'url': '/page/second',
            'popularity': 2,
            'title': 'This is four items',
        }, headers={'Auth-Key': 'xyz123'})
        r = self.post('/v1', {
            'url': '/page/third',
            'popularity': 3,
            'title': 'Fourier thinking',
        }, headers={'Auth-Key': 'xyz123'})

        r = self.get('/v1?q=four&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 3)

        r = self.get('/v1?q=four%20thin&d=peterbecom')
        eq_(r.status_code, 200)
        eq_(len(r.json()['results']), 1)

    def test_domain_counter_increments(self):
        r = self.post('/v1', {
            'url': ' /plog/something   ',
            'popularity': "12",
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)

        r = self.get('/v1?q=b&d=peterbecom')
        assert r.json()['results']
        now = datetime.datetime.utcnow()
        year_key = '$domainfetches$%s' % now.year
        month_key = year_key + '$%s' % now.month
        eq_(int(self.c.hget(month_key, 'peterbecom')), 1)

        r = self.get('/v1?q=bl&d=peterbecom')
        assert r.json()['results']
        eq_(int(self.c.hget(month_key, 'peterbecom')), 2)

    def test_get_stats(self):
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        now = datetime.datetime.utcnow()
        eq_(stats['fetches'], {str(now.year): {}})
        eq_(stats['documents'], 0)

        r = self.post('/v1', {
            'url': ' /plog/something   ',
            'popularity': "12",
            'title': "This is a blog about something",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1?q=b&d=peterbecom')
        assert r.json()['results']
        r = self.get('/v1?q=bl&d=peterbecom')
        assert r.json()['results']

        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        now = datetime.datetime.utcnow()
        eq_(stats['fetches'][str(now.year)][str(now.month)], 2)
        eq_(stats['documents'], 1)

        # do an edit. different title same url
        r = self.post('/v1', {
            'url': '/plog/something',
            'popularity': "13",
            'title': "This is a blog about something extra",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 1)

        # new document
        r = self.post('/v1', {
            'url': '/plog/different',
            'popularity': "99",
            'title': "Something different",
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 201)
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 2)

        # remove that last one
        r = self.delete('/v1', params={
            'url': '/plog/different',
        }, headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 204)
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 1)

    def test_bulk_upload(self):
        documents = [
            {
                'url': '/some/page',
                'title': 'Some title',
                'popularity': 10
            },
            {
                'url': '/other/page',
                'title': 'Other title',
            },
            {
                'url': '/private/page',
                'title': 'Other private page',
                'group': 'private'
            },
        ]
        r = self.post(
            '/v1/bulk',
            data=json.dumps({'documents': documents}),
            headers={
                'Auth-Key': 'xyz123',
                # 'content-type': 'application/json'
            }
        )
        eq_(r.status_code, 201)
        r = self.get('/v1/stats', headers={'Auth-Key': 'xyz123'})
        eq_(r.status_code, 200)
        stats = r.json()
        eq_(stats['documents'], 3)

        r = self.get('/v1?q=titl&d=peterbecom')
        eq_(len(r.json()['results']), 2)
        urls = [x[0] for x in r.json()['results']]
        eq_(urls, ['/some/page', '/other/page'])
        r = self.get('/v1?q=other&d=peterbecom&g=private')
        eq_(len(r.json()['results']), 2)
