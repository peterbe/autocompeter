from django.conf import settings


def auth0(request):
    return {
        'AUTH0_CLIENT_ID': settings.AUTH0_CLIENT_ID,
        'AUTH0_DOMAIN': settings.AUTH0_DOMAIN,
    }
