import json

from elasticsearch_dsl.connections import connections

from django.core.urlresolvers import reverse
from django.test import TestCase
from django.core.management import call_command
from django.contrib.auth.models import User
from django.utils import timezone

from autocompeter.main.models import Domain, Key, Search


class IntegrationTestBase(TestCase):

    def setUp(self):
        super(IntegrationTestBase, self).setUp()
        call_command('create-index', verbosity=0, interactive=False)
        status = connections.get_connection().cluster.health()['status']
        assert status == 'green', status

    @staticmethod
    def _refresh():
        connections.get_connection().indices.refresh()


class TestIntegrationAPI(IntegrationTestBase):

    def post_json(self, url, payload=None, **extra):
        payload = payload or {}
        extra['content_type'] = 'application/json'
        return self.client.post(url, json.dumps(payload), **extra)

    def test_happy_path_search(self):
        url = reverse('api:home')

        # This won't work because the domain is not recognized yet
        response = self.client.get(url, {
            'q': 'fo',
            'd': 'example.com',
        })
        self.assertEqual(response.status_code, 400)

        domain = Domain.objects.create(name='example.com')
        key = Key.objects.create(
            domain=domain,
            key='mykey',
            user=User.objects.create(username='dude'),
        )
        response = self.client.get(url, {
            'q': 'fo',
            'd': 'example.com',
        })
        self.assertEqual(response.status_code, 200)
        self.assertTrue(not response.json()['results'])

        # Index some.

        # First without a key
        response = self.client.post(url, {
            'title': 'Foo Bar One',
            'url': 'https://example.com/one',
        })
        self.assertEqual(response.status_code, 400)

        # Now with an invalid key
        response = self.client.post(url, {
            'title': 'Foo Bar One',
            'url': 'https://example.com/one',
        }, HTTP_AUTH_KEY="junk")
        self.assertEqual(response.status_code, 403)

        # This time with a valid key!
        response = self.client.post(url, {
            'title': 'Foo Bar One',
            'url': 'https://example.com/one',
        }, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 201)

        response = self.client.post(url, {
            'title': 'Foo Bar Two',
            'url': 'https://example.com/two',
            'popularity': 2
        }, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 201)

        response = self.client.post(url, {
            'title': 'Foo Bar Private',
            'url': 'https://example.com/private',
            'group': 'private',
        }, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 201)

        self._refresh()

        # Now do a search
        response = self.client.get(url, {
            'q': 'foo',
            'd': 'example.com',
        })
        self.assertEqual(response.status_code, 200)
        results = response.json()['results']
        assert results
        self.assertEqual(
            results,
            [
                # higher popularity
                ['https://example.com/two', 'Foo Bar Two'],
                # no specific group
                ['https://example.com/one', 'Foo Bar One'],
            ]
        )

        # With groups
        response = self.client.get(url, {
            'q': 'priv',
            'd': 'example.com',
            'g': 'private'
        })
        self.assertEqual(response.status_code, 200)
        results = response.json()['results']
        assert results
        self.assertEqual(
            results,
            [
                ['https://example.com/private', 'Foo Bar Private']
            ]
        )

        # Overwrite one
        response = self.client.post(url, {
            'title': 'Foo Bar One Hundred and One',
            'url': 'https://example.com/one',
            'popularity': 100,
        }, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 201)
        self._refresh()
        response = self.client.get(url, {
            'q': 'foo',
            'd': 'example.com',
        })
        self.assertEqual(response.status_code, 200)
        results = response.json()['results']
        assert results
        self.assertEqual(
            results,
            [
                # no specific group
                ['https://example.com/one', 'Foo Bar One Hundred and One'],
                # now lower popularity
                ['https://example.com/two', 'Foo Bar Two'],
            ]
        )

        # Delete one of them
        response = self.client.delete(
            url + '?url=https://example.com/private',
            HTTP_AUTH_KEY=key.key
        )
        self.assertEqual(response.status_code, 200)
        self._refresh()

        # Get some stats on these searches
        url = reverse('api:stats')
        response = self.client.get(url)
        self.assertEqual(response.status_code, 400)
        response = self.client.get(url, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 200)
        results = response.json()
        self.assertEqual(results['documents'], 3 - 1)
        now = timezone.now()
        fetches = {
            str(now.year): {
                str(now.month): 4  # because we've done 4 searches
            }
        }
        self.assertEqual(Search.objects.filter(domain=domain).count(), 4)
        self.assertEqual(fetches, results['fetches'])

        # Flush out all
        url = reverse('api:flush')
        response = self.client.post(url, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 200)
        self._refresh()
        # Get the stats again
        url = reverse('api:stats')
        response = self.client.get(url, HTTP_AUTH_KEY=key.key)
        self.assertEqual(response.status_code, 200)
        results = response.json()
        self.assertEqual(results['documents'], 0)

    def test_bulk_load(self):
        key = Key.objects.create(
            domain=Domain.objects.create(name='example.com'),
            key='mykey',
            user=User.objects.create(username='dude'),
        )
        documents = [
            {
                'url': 'http://example.com/one',
                'title': 'Zebra One',
            },
            {
                'url': 'http://example.com/two',
                'title': 'Zebra Two',
                'popularity': 100,
            },
            {
                'url': 'http://example.com/private',
                'title': 'Zebra Private',
                'group': 'private'
            },
            {
                'title': 'No URL!!',
            },
            {
                'url': 'No title!!',
            },
        ]
        url = reverse('api:bulk')
        response = self.post_json(url, {'documents': documents})
        self.assertEqual(response.status_code, 400)

        response = self.post_json(
            url,
            {'documents': documents},
            HTTP_AUTH_KEY='junk'
        )
        self.assertEqual(response.status_code, 403)
        response = self.post_json(
            url,
            {'no documents': 'key'},
            HTTP_AUTH_KEY=key.key
        )
        self.assertEqual(response.status_code, 400)
        response = self.post_json(
            url,
            {'documents': documents},
            HTTP_AUTH_KEY=key.key
        )
        self.assertEqual(response.status_code, 201)
        result = response.json()
        # the one without url and the one without title skipped
        self.assertEqual(result['count'], 3)

        self._refresh()

        url = reverse('api:home')
        response = self.client.get(url, {
            'q': 'zebrra',
            'd': 'example.com',
        })
        self.assertEqual(response.status_code, 200)
        _json = response.json()
        terms = _json['terms']
        self.assertTrue('zebrra' in terms)
        self.assertTrue('zebra' in terms)
        results = _json['results']
        assert results, _json
        self.assertEqual(
            results,
            [
                ['http://example.com/two', 'Zebra Two'],
                ['http://example.com/one', 'Zebra One'],
            ]
        )
