#!/bin/bash
# pwd is the git repo.
set -e

export CACHE_BACKEND="django.core.cache.backends.locmem.LocMemCache"
export DATABASE_URL="postgres://travis:@localhost/autocompeter"
export SECRET_KEY="anything"
export ALLOWED_HOSTS="localhost"

echo "Running collectstatic"
python manage.py collectstatic --noinput
