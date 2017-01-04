from django.conf.urls import url

from autocompeter.authentication import views


urlpatterns = [
    url(
        r'^callback/?$',
        views.callback,
        name='callback'
    ),
    url(
        r'^signout/$',
        views.signout,
        name='signout'
    ),
]
