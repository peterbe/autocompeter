from elasticsearch_dsl.connections import connections

from django.conf import settings
from django.apps import AppConfig


class MainConfig(AppConfig):
    name = 'autocompeter.main'

    def ready(self):
        connections.configure(**settings.ES_CONNECTIONS)
