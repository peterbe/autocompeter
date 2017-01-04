#!/bin/bash
# pwd is the git repo.
set -e

curl -v http://localhost:9200/

export CACHE_BACKEND="django.core.cache.backends.locmem.LocMemCache"
export DATABASE_URL="postgres://travis:@localhost/autocompeter"
export SECRET_KEY="anything"
export ALLOWED_HOSTS="localhost"

python manage.py test
