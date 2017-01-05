import hashlib
import base64
from urllib.parse import urlencode

import requests
from requests.exceptions import ConnectTimeout, ReadTimeout

from django import http
from django.conf import settings
from django.contrib import messages
from django.shortcuts import redirect
from django.utils.encoding import smart_bytes
from django.contrib.auth import get_user_model, login, logout
from django.views.decorators.http import require_POST


User = get_user_model()


def callback(request):
    code = request.GET.get('code', '')
    if not code:
        # If the user is blocked, we will never be called back with a code.
        # What Auth0 does is that it calls the callback but with extra
        # query string parameters.
        if request.GET.get('error'):
            messages.error(
                request,
                "Unable to sign in because of an error from Auth0. "
                "({})".format(
                    request.GET.get(
                        'error_description',
                        request.GET['error']
                    )
                )
            )
            return redirect('main:home')
        return http.HttpResponseBadRequest("Missing 'code'")
    token_url = 'https://{}/oauth/token'.format(settings.AUTH0_DOMAIN)

    callback_url = settings.AUTH0_CALLBACK_URL
    if '://' not in callback_url:
        callback_url = '{}://{}{}'.format(
            'https' if request.is_secure() else 'http',
            request.site.domain,
            callback_url,
        )
    token_payload = {
        'client_id': settings.AUTH0_CLIENT_ID,
        'client_secret': settings.AUTH0_CLIENT_SECRET,
        'redirect_uri': callback_url,
        'code': code,
        'grant_type': 'authorization_code',
    }
    try:
        token_info = requests.post(
            token_url,
            json=token_payload,
            timeout=settings.AUTH0_PATIENCE_TIMEOUT,
        ).json()
    except (ConnectTimeout, ReadTimeout):
        raise
        messages.error(
            request,
            'Unable to authenticate with Auth0. The Auth0 service timed out. '
            'This is most likely temporary so you can try again in a couple '
            'of minutes.'
        )
        return redirect('main:home')

    if not token_info.get('access_token'):
        messages.error(
            request,
            'Unable to authenticate with Auth0. Most commonly this '
            'happens because the authentication token has expired. '
            'Please refresh and try again.'
        )
        return redirect('main:home')

    user_url = 'https://{}/userinfo'.format(
        settings.AUTH0_DOMAIN,
    )
    user_url += '?' + urlencode({
        'access_token': token_info['access_token'],
    })
    try:
        user_response = requests.get(
            user_url,
            timeout=settings.AUTH0_PATIENCE_TIMEOUT,
        )
    except (ConnectTimeout, ReadTimeout):
        messages.error(
            request,
            'Unable to authenticate with Auth0. The Auth0 service timed out. '
            'This is most likely temporary so you can try again in a couple '
            'of minutes.'
        )
        return redirect('main:home')
    if user_response.status_code != 200:
        messages.error(
            request,
            'Unable to retrieve user info from Auth0 ({}, {!r})'.format(
                user_response.status_code,
                user_response.text
            )
        )
        return redirect('main:home')

    user_info = user_response.json()
    assert user_info['email'], user_info

    if not user_info['email_verified']:
        messages.error(
            request,
            'Email {} not verified.'.format(
                user_info['email']
            )
        )
        return redirect('main:home')
    user = get_user(user_info)

    if not user.is_active:
        messages.error(
            request,
            "User account ({}) found but it has been made inactive.".format(
                user.email,
            )
        )
        return redirect('main:home')
    else:
        user.backend = 'django.contrib.auth.backends.ModelBackend'
        login(request, user)
        messages.success(
            request,
            'Signed in with email: {}'.format(user.email)
        )
        return redirect(settings.AUTH0_SUCCESS_URL)


def default_username(email):
    # Store the username as a base64 encoded sha1 of the email address
    # this protects against data leakage because usernames are often
    # treated as public identifiers (so we can't use the email address).
    return base64.urlsafe_b64encode(
        hashlib.sha1(smart_bytes(email)).digest()
    ).rstrip(b'=')


def get_user(user_info):
    email = user_info['email']

    try:
        return User.objects.get(email__iexact=email)
    except User.DoesNotExist:
        return User.objects.create(
            email=email,
            username=default_username(email),
            first_name=user_info.get('given_name') or '',
            last_name=user_info.get('family_name') or '',
        )


@require_POST
def signout(request):
    signout_url = settings.AUTH0_SIGNOUT_URL
    if '://' not in signout_url:
        signout_url = '{}://{}{}'.format(
            'https' if request.is_secure() else 'http',
            request.site.domain,
            signout_url,
        )
    logout(request)
    messages.success(request, 'Signed out')
    url = 'https://' + settings.AUTH0_DOMAIN + '/v2/logout'
    url += '?' + urlencode({
        'returnTo': signout_url,
        'client_id': settings.AUTH0_CLIENT_ID,
    })
    return redirect(url)
