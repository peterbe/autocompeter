from django.conf.urls import url

from autocompeter.main import views


urlpatterns = [
    url(
        r'^$',
        views.home,
        name='home'
    ),
]
