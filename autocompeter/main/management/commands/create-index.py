from django.core.management.base import BaseCommand

from autocompeter.main.search import index


class Command(BaseCommand):

    def handle(self, **options):
        index.delete(ignore=404)
        index.create()
