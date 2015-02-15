A Go autocomplete Redis server.
===============================

[![Build Status](https://travis-ci.org/peterbe/autocompeter.svg?branch=master)](https://travis-ci.org/peterbe/autocompeter)

Running tests
-------------

Both the unit test and integration test will connect to **Redis on
database 8**. It will do a flush on this database.

To run the unit tests run:

    go test -v

To run the end-to-end test run:

    nosetests

To run the python end-to-end tests you need to install some dependencies:

    pip install -r requirements.txt

Database structure
------------------

First we take the domain and hash it into a unique 8 character string.
Let's call this `E`.

Secondly we take the URL and hash it into a unique 8 character string too.
Let's call this `U`. The un-encoded URL we'll called `URL`.

The popularity is a floating point number. Let's call it `P`.

For every prefix of every word in every title, e.g. "w", "wo", "wor", "word",
we store:

    ZADD E+w P U
    ZADD E+wo P U
    ZADD E+wor P U
    ZADD E+word P U

When you use `ZADD` you're basically storing a "sorted set". I.e. the
keys are never repeated (if you two two ZADD with the same key only).

The equivalent in Python would be something like this:

    urls = {}
    urls[U] = []
    urls[U].append((P, E+w))
    urls[U].append((P, E+wo))
    urls[U].append((P, E+wor))
    urls[U].append((P, E+word))

But unlike Python, in Redis you can reverse the search by a key, e.g `E+wor`
and get **all** URLs sorted by their score.

Next we store a simple hash table of the encoded URL to the un-encoded URL.
We do this under a key specific to the domain:

    HSET E+$urls U URL

The difference between `HSET` and `HMSET` is that you can do multiples with
`HMSET` in one sweep. E.g.:

    HMSET E+$urls U1 URL1 U2 URL2 Un URLn

Since the scoring search we use gives us the encoded URL (`U`) we'll use
this hash table to retrieve back to the full un-encoded URL (`URL`).

The equivalent in Python would be something like:

    urls = {}
    urls[E+$urls] = {}
    urls[E+$urls][U1] = URL1
    urls[E+$urls][U2] = URL2
    urls[E+$urls][Un] = URLn

Lastly we store a hash table of the encoded URLs to the title (`T`). This is
used so we can retrieve the title back from the encoded URL (`U`).

    HSET E+$titles U T

The equivalent in Python would be:

    titles = {}
    titles[E+$titles] = {}
    titles[E+$titles][U1] = T1
    titles[E+$titles][U2] = T2
    titles[E+$titles][Un] = Tn

At this point, to retrieve a title (`T`) back from an encoded URL (`U`)

Now, if we want to reverse the storage, e.g. remove a URL+title record, we
first need to go from URL to title so we can get all its prefixes.

Now we can reverse every ZADD that was done without knowing the popularity.
This you do with a `ZREM` and simply knowing every prefix and its encoded URL.

Once we've removed each prefix, we can remove the title by that encoded URL.


Writing documentation
---------------------

If you want to work on the documentation, cd into the directory `./doc`
and make sure you have `mkdocs` pip installed. (see
`./requirements.txt`).

Then simple run:

    mkdocs build
    open site/index.html

If you have a bunch of changes you want to make and don't want to run
`mkdocs build` every time you can use this trick:

    mkdocs serve
    open http://localhost:8000/
