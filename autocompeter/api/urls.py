from django.conf.urls import url

from autocompeter.api import views


urlpatterns = [
    url(
        r'^bulk$',
        views.bulk,
        name='bulk'
    ),
    url(
        r'^ping$',
        views.ping,
        name='ping'
    ),
    url(
        r'^stats$',
        views.stats,
        name='stats'
    ),
    url(
        r'^flush$',
        views.flush,
        name='flush'
    ),
    url(
        r'',
        views.home,
        name='home'
    ),
]
