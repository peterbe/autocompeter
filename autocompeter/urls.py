from django.conf.urls import url, include
from django.conf import settings
from django.conf.urls.static import static

import autocompeter.main.urls
import autocompeter.api.urls
import autocompeter.authentication.urls


urlpatterns = [
    url(
        '',
        include(autocompeter.main.urls.urlpatterns, namespace='main')
    ),
    url(
        r'^auth/',
        include(autocompeter.authentication.urls.urlpatterns, namespace='auth')
    ),
    url(
        r'^v1/?',
        include(autocompeter.api.urls.urlpatterns, namespace='api')
    ),
]

if settings.DEBUG:
    urlpatterns += static(
        settings.STATIC_URL,
        document_root=settings.STATIC_ROOT
    )
