# Autocompeter

How to get an awesome autocomplete/live-search widget on your own site.

## The Javascript

### Regular download

You can either download the javascript code and put it into your own site
or you can leave it to Autocompeter to host it on its CDN (backed by
AWS CloudFront). To download, go to:

[**https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.min.js**](https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.min.js)

This is the optimized version. If you want to, you can download [the
non-minified file](https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.js) instead which is easier to debug or hack on.

### Bower (to download)

You can use [Bower](http://bower.io/) to download the code too.
This will only install the files in a local directory called
`bower_components`. This is how you download and install with Bower:

    bower install autocompeter
    ls bower_components/autocompeter/public/dist/

It's up to you if you leave it there or if you copy those files where you
prefer them to be.

### Hosted

To use our CDN the URL is:

[http://cdn.autocompeter.com/dist/autocompeter-v1.min.js](http://cdn.autocompeter.com/dist/autocompeter-v1.min.js) (or [https version](https://cdn.autocompeter.com/dist/autocompeter-v1.min.js))

But it's recommended that when you insert this URL into your site, instead
of prefixing it with `http://` or `https://` prefix it with just `//`. E.g.
like this:

    <script src="//cdn.autocompeter.com/dist/autocompeter-v1.min.js"></script>

### Configure the Javascript

So, for the widget to work you need to have an `<input type="text">` field
somewhere. You can put an attributes you like on it like `name="something"`
or `class="mysearch"` for example. But you need to be able to retrieve it as
a HTML DOM node because when you activate `Autocompeter` the first and only
required argument is the input DOM node. For example:

    <script>
    Autocompeter(document.getElementByClass('mysearch')[0]);
    </script>

or...

    <script>
    Autocompeter(document.querySelector('input.mysearch'));
    </script>

By default it uses the domain that is currently being used. It retrieves this
by simply using `window.location.host`. If you for example, have a dev or
staging version of your site but you want to use the production domain, you
can manually set the domain like this:

    <script>
    Autocompeter(document.querySelector('input.mysearch'), {
      domain: 'example.com'
    });
    </script>

There are a couple of other options you can override. For example the
maximum number of items to be returned. The default is 10.:

    <script>
    Autocompeter(document.querySelector('input.mysearch'), {
      domain: 'example.com',
      number: 20
    });
    </script>

Another thing you might want to set is the `groups` parameter. Note that
this can be an array or a comma separated string. Suppose that this information
is set by the server-side rendering, you can use it like this for example,
assuming here some server-side template rendering code like Django or Jinja
for example:

    <script>
    var groups = [];
    {% if user.is_logged_in %}
    groups.push('private');
    {% if user.is_admin %}
    groups.push('admins');
    {% endif %}
    {% endif %}
    Autocompeter(document.querySelector('input.mysearch'), {
      domain: 'example.com',
      number: 20,
      groups: groups
    });
    </script>

So, suppose you set `groups: "private,admins"` it will still search on titles
that were defined with no group. Doing it like this will just return
potentially more titles.

## The CSS

Just like with downloading the Javascript, you can do with the CSS.

[**https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.min.css**](https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.min.css)

Or...

    bower install autocompeter
    ls bower_components/autocompeter/public/dist/*.css

Or...

    <link rel="stylesheet" href="//cdn.autocompeter.com/dist/autocompeter-v1.min.css">

There is also another alternative. If you already use [Sass (aka. SCSS)](http://sass-lang.com/)
you can download [autocompeter.scss](https://github.com/peterbe/autocompeter/blob/master/src/autocompeter.scss)
instead and incorporate that into your own build system.

### Overriding

It's very possible that on your site, the CSS doesn't fit in perfectly. Either
you don't exactly like the way it looks or it just doesn't work as expected.
The recommended way to deal with this is to override certain selectors. For
example it might look like this:

    <link rel="stylesheet" href="//cdn.autocompeter.com/dist/autocompeter-v1.min.css">
    <style>
    ._ac-wrap { width: 400px; }
    @media only screen and (max-width : 321px) {
      ._ac-wrap { width: 290px; }
    }
    </style>

As an example, with the design being used on
[autocompeter.com](http://autocompeter.com) some CSS had to be overridden.

## The API

### Getting an Auth Key

This is under construction. There is currently no automated way to generate
an auth key.

Every Auth Key belongs to one single domain. E.g. `www.peterbe.com`.

### Submitting titles

You have to submit one title at a time. (This might change in the near future)

You'll need an Auth Key, a title, a URL, optionally a popularity number and
optionally a group for access control..

The URL you need to do a **HTTP POST** to is:

    http://autocompeter.com/v1

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
    http://autocompeter.com/v1

Here's the same example using Python [requests](http://requests.readthedocs.org/):

    response = requests.post(
        'http://autocompeter.com/v1',
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

### Uniqueness of the URL

You can submit two "documents" that have the same title but you can not submit
two documents that have the same URL. If you submit:

    curl -X POST -H "Auth-Key: yoursecurekey" \
    -d url=http://www.example.com/page.html \
    -d title="This is the first title" \
    http://autocompeter.com/v1
    # now the same URL, different title
    curl -X POST -H "Auth-Key: yoursecurekey" \
    -d url=http://www.example.com/page.html \
    -d title="A different title the second time" \
    http://autocompeter.com/v1

Then, the first title will be overwritten and replaced with the second title.

### About the popularity

If you omit the `popularity` key, it's the same as sending it as `0`.

The search will always be sorted by the `popularity` and the higher the number
the higher the document title will appear in the search results.

If you don't really have the concept of ranking your titles by a popularity
or hits or score or anything like that, then use the titles "date" so that
the most recent ones have higher priority. That way more fresh titles appear
first.

### About the groups and access control and privacy

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

### How to delete a title/URL

If a URL hasn't changed by the title has, you can simply submit it again.
Or if neither the title or the URL has changed but the popularity has changed
you can simply submit it again.

However, suppose a title needs to be remove you send a **HTTP DELETE**. Send
 it to the same URL you use to submit a title. E.g.

    curl -X DELETE -H "Auth-Key: yoursecurekey" \
    http://autocompeter.com/v1?url=http%3A//www.example.com/page.html

Note that you can't use `application/x-www-form-urlencoded` with HTTP DELETE.
So you have to put the `?url=...` into the URL.

Note also that in this example the `url` is URL encoded. The `:` becomes `%3A`.
