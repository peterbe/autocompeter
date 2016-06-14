# The API

## Getting an Auth Key

To generate an authentication key, you have to go to
[autocompeter.com](https://autocompeter.com/#login) and sign in using GitHub.

Once you've done that you get access to a form where you can type in your
domain name and generate a key. Copy-n-paste that somewhere secure and use
when you access private API endpoints.

Every Auth Key belongs to one single domain.
E.g. `yoursecurekey->www.peterbe.com`.

## Submitting titles

You have to submit one title at a time. (This might change in the near future)

You'll need an Auth Key, a title, a URL, optionally a popularity number and
optionally a group for access control..

The URL you need to do a **HTTP POST** to is:

    https://autocompeter.com/v1

The Auth Key needs to be set as a HTTP header called `Auth-Key`.

The parameters need to be sent as `application/x-www-form-urlencoded`.

The keys you need to send are:

| Key          | Required | Example                          |
|--------------|----------|----------------------------------|
| `title`      | Yes      | A blog post example              |
| `url`        | Yes      | http://www.example.com/page.html |
| `group`      | No       | loggedin                         |
| `popularity` | No       | 105                              |

Here's an example using `curl`:

    curl -X POST -H "Auth-Key: yoursecurekey"
    -d url=http://www.example.com/page.html \
    -d title="A blog post example" \
    -d group="loggedin" \
    -d popularity="105" \
    https://autocompeter.com/v1

Here's the same example using Python [requests](https://requests.readthedocs.io/):

    response = requests.post(
        'https://autocompeter.com/v1',
        data={
            'title': 'A blog post example',
            'url': 'http://www.example.com/page.html',
            'group': 'loggedin',
            'popularity': 105
        },
        headers={
            'Auth-Key': 'yoursecurekey'
        }
    )
    assert response.status_code == 201

The response code will always be `201` and the response content will be
`application/json` that simple looks like this:

    {"message": "OK"}

## Uniqueness of the URL

You can submit two "documents" that have the same title but you can not submit
two documents that have the same URL. If you submit:

    curl -X POST -H "Auth-Key: yoursecurekey" \
    -d url=http://www.example.com/page.html \
    -d title="This is the first title" \
    https://autocompeter.com/v1
    # now the same URL, different title
    curl -X POST -H "Auth-Key: yoursecurekey" \
    -d url=http://www.example.com/page.html \
    -d title="A different title the second time" \
    https://autocompeter.com/v1

Then, the first title will be overwritten and replaced with the second title.

## About the popularity

If you omit the `popularity` key, it's the same as sending it as `0`.

The search will always be sorted by the `popularity` and the higher the number
the higher the document title will appear in the search results.

If you don't really have the concept of ranking your titles by a popularity
or hits or score or anything like that, then use the titles "date" so that
the most recent ones have higher priority. That way more fresh titles appear
first.

## About the groups and access control and privacy

Suppose your site visitors should see different things depending how they're
signed in. Well, first of all you **can't do it on per-user basis**.

However, suppose you have a set of titles for all visitors of the site
and some extra just for people who are signed in, then you can use `group`
as a parameter per title.

**Note: There is no way to securely protect this information. You can
make it so that restricted titles don't appear to people who shouldn't see
it but it's impossible to prevent people from manually querying by a
specific group on the command line for example. **

Note that you can have multiple groups. For example, the titles that is
publically available you submit with no `group` set (or leave it as
an empty string) and then you submit some as `group="private"` and some
as `group="admins"`.

## How to delete a title/URL

If a URL hasn't changed by the title has, you can simply submit it again.
Or if neither the title or the URL has changed but the popularity has changed
you can simply submit it again.

However, suppose a title needs to be remove you send a **HTTP DELETE**. Send
 it to the same URL you use to submit a title. E.g.

    curl -X DELETE -H "Auth-Key: yoursecurekey" \
    https://autocompeter.com/v1?url=http%3A//www.example.com/page.html

Note that you can't use `application/x-www-form-urlencoded` with HTTP DELETE.
So you have to put the `?url=...` into the URL.

Note also that in this example the `url` is URL encoded. The `:` becomes `%3A`.

## How to remove all your documents

You can start over and flush all the documents you have sent it by doing
a HTTP DELETE request to the url `/v1/flush`. Like this:

    curl -X DELETE -H "Auth-Key: yoursecurekey" \
    https://autocompeter.com/v1/flush

This will reset the counts all related to your domain. The only thing that
isn't removed is your auth key.

## Bulk upload

Instead of submitting one "document" at a time you can instead send in a
whole big JSON blob. The struct needs to be like this example:

    {
        "documents": [
            {
                "url": "http://example.com/page1",
                "title": "Page One"
            },
            {
                "url": "http://example.com/page2",
                "title": "Other page",
                "popularity": 123
            },
            {
                "url": "http://example.com/page3",
                "title": "Last page",
                "group": "admins"
            },
        ]
    }

Note that the `popularity` and the `group` keys are optional. Each
dictionary in the array called `documents` needs to have a `url` and `title`.

The endpoint to use `https://autocompeter.com/v1/bulk` and you need to do a
HTTP POST or a HTTP PUT.

Here's an example using curl:

    curl -X POST -H "Auth-Key: 3b14d7c280bf525b779d0a01c601fe44" \
    -d '{"documents": [{"url":"/url", "title":"My Title", "popularity":1001}]}' \
    https://autocompeter.com/v1/bulk

And here's an example using
Python [requests](https://requests.readthedocs.io/en/latest/):


```python
import json
import requests

documents = [
    {
        'url': '/some/page',
        'title': 'Some title',
        'popularity': 10
    },
    {
        'url': '/other/page',
        'title': 'Other title',
    },
    {
        'url': '/private/page',
        'title': 'Other private page',
        'group': 'private'
    },
]
print requests.post(
    'https://autocompeter.com/v1/bulk',
    data=json.dumps({'documents': documents}),
    headers={
        'Auth-Key': '3b14d7c280bf525b779d0a01c601fe44',
    }
)
```
