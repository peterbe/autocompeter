from django.apps import AppConfig
from django.conf import settings


class AuthConfig(AppConfig):
    name = 'auth'

    def ready(self):
        assert settings.AUTH0_CLIENT_SECRET
