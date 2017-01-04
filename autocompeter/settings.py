import os
import sys

from decouple import config, Csv
from unipath import Path
import dj_database_url


BASE_DIR = Path(__file__).parent
# See https://docs.djangoproject.com/en/1.10/howto/deployment/checklist/

SECRET_KEY = config('SECRET_KEY')

DEBUG = config('DEBUG', default=False, cast=bool)
DEBUG_PROPAGATE_EXCEPTIONS = config(
    'DEBUG_PROPAGATE_EXCEPTIONS',
    False,
    cast=bool,
)

ALLOWED_HOSTS = config('ALLOWED_HOSTS', cast=Csv())

# Application definition

INSTALLED_APPS = [
    'django.contrib.auth',
    'django.contrib.contenttypes',
    'django.contrib.sessions',
    'django.contrib.messages',
    'django.contrib.postgres',
    'django.contrib.staticfiles',

    'autocompeter.main',
    'autocompeter.api',
]

MIDDLEWARE = [
    'django.middleware.security.SecurityMiddleware',
    'django.contrib.sessions.middleware.SessionMiddleware',
    'django.middleware.common.CommonMiddleware',
    'django.middleware.csrf.CsrfViewMiddleware',
    'django.contrib.auth.middleware.AuthenticationMiddleware',
    'django.contrib.messages.middleware.MessageMiddleware',
    'django.middleware.clickjacking.XFrameOptionsMiddleware',
    'django.contrib.sites.middleware.CurrentSiteMiddleware',
]

ROOT_URLCONF = 'autocompeter.urls'

TEMPLATES = [
    {
        'BACKEND': 'django.template.backends.django.DjangoTemplates',
        'DIRS': [],
        'APP_DIRS': True,
        'OPTIONS': {
            'context_processors': [
                'django.template.context_processors.debug',
                'django.template.context_processors.request',
                'django.contrib.messages.context_processors.messages',
                'autocompeter.authentication.context_processors.auth0',
            ],
        },
    },
]

WSGI_APPLICATION = 'autocompeter.wsgi.application'


DATABASES = {
    'default': config(
        'DATABASE_URL',
        default='postgres://postgres@db:5432/postgres',
        cast=dj_database_url.parse
    )
}

CACHES = {
    'default': {
        'BACKEND': config(
            'CACHE_BACKEND',
            'django.core.cache.backends.memcached.MemcachedCache',
        ),
        'LOCATION': config('CACHE_LOCATION', '127.0.0.1:11211'),
        'TIMEOUT': config('CACHE_TIMEOUT', 500),
        'KEY_PREFIX': config('CACHE_KEY_PREFIX', 'autocompeter'),
    }
}


LANGUAGE_CODE = 'en-us'

TIME_ZONE = 'UTC'

USE_I18N = False

USE_L10N = False

USE_TZ = True


SESSION_COOKIE_AGE = 60 * 60 * 24 * 365


# Static files (CSS, JavaScript, Images)
# https://docs.djangoproject.com/en/1.10/howto/static-files/

STATIC_URL = '/static/'

STATICFILES_DIRS = [
    os.path.abspath(os.path.join(BASE_DIR, '../public/dist')),
]


STATIC_ROOT = os.path.join(BASE_DIR, 'static')


# ElasticSearch

ES_INDEX = 'autocompeter'

ES_INDEX_SETTINGS = {
    'number_of_shards': 1,
    'number_of_replicas': 0,
}

ES_CONNECTIONS = {
    'default': {
        'hosts': config(
            'ES_CONNECTIONS_URLS',
            default='localhost:9200',
            cast=Csv()
        )
    },
}

AUTH0_CLIENT_ID = config('AUTH0_CLIENT_ID', 'FCTzKEnD2H88IuYWJredjYH6fWgp0FlM')
AUTH0_DOMAIN = config('AUTH0_DOMAIN', 'peterbecom.auth0.com')
AUTH0_CALLBACK_URL = config('AUTH0_CALLBACK_URL', '/auth/callback')
AUTH0_SIGNOUT_URL = config('AUTH0_SIGNOUT_URL', '/')
AUTH0_SUCCESS_URL = config('AUTH0_SUCCESS_URL', 'main:home')
AUTH0_CLIENT_SECRET = config('AUTH0_CLIENT_SECRET', '')
AUTH0_PATIENCE_TIMEOUT = config('AUTH0_PATIENCE_TIMEOUT', 5, cast=int)


if 'test' in sys.argv[1:2]:
    # os.environ['OPBEAT_DISABLE_SEND'] = 'true'
    CACHES = {
        'default': {
            'BACKEND': 'django.core.cache.backends.locmem.LocMemCache',
            'LOCATION': 'unique-snowflake',
        }
    }
    ES_INDEX = 'test_autocompeter'
