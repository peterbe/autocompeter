import time
from collections import defaultdict
from urllib import urlencode
from random import shuffle

import requests


URL1 = 'http://localhost:8000/autocomplete'
URL2 = 'http://localhost:3000/v1'

def run():
    searches = [
        'xhtml',
        'html',
        'xhtml css',
        '[xhtml] (css) !',
        "o'clock",
        "text",
        "sql s",
        "sql sc",
        "python s",
        "python st",
        "python sa",
    ]
    def all(word):
        for i in range(1, len(word) + 1):
            searches.append(word[:i])
    all('python')
    all('DJANGO')
    all('extra')
    all('SQL')
    all('textarea')
    all('zope')
    shuffle(searches)
    searches += searches
    searches += searches
    times = defaultdict(list)
    for search in searches:
        q = urlencode({'q': search})
        for base in (URL1, URL2):
            print base
            url = '%s?%s' % (base, q)
            t0 = time.time()
            result = requests.get(url)
            t1 = time.time()
            times[base].append(t1-t0)
            assert result.status_code == 200, url
            print result.json()
            print

    for base, msecs in times.items():
        print base
        print len(msecs), "searches"
        print sum(msecs)


if __name__ == '__main__':
    import sys
    sys.exit(run())
