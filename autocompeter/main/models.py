from django.db import models
from django.contrib.auth.models import User


class Domain(models.Model):
    name = models.CharField(max_length=100)
    created = models.DateTimeField(auto_now_add=True)
    modified = models.DateTimeField(auto_now=True)

    def __str__(self):
        return self.name


class Key(models.Model):
    domain = models.ForeignKey(Domain)
    key = models.TextField(db_index=True, unique=True)
    user = models.ForeignKey(User)
    modified = models.DateTimeField(auto_now=True)

    def __str__(self):
        return self.key


class Search(models.Model):
    domain = models.ForeignKey(Domain)
    term = models.TextField()
    results = models.IntegerField(default=0)
    created = models.DateTimeField(auto_now_add=True)
