An ElasticSearch autocomplete Django server
===========================================

[![Build Status](https://travis-ci.org/peterbe/autocompeter.svg?branch=master)](https://travis-ci.org/peterbe/autocompeter)

Documentation
-------------

[Documentation on Read the Docs](https://autocompeter.readthedocs.io)

Running tests
-------------

To run the unit tests run:

    ./manage.py test

Using Docker
------------

First you need to create your own `.env` file. It should look something like
this:

    DEBUG=True
    SECRET_KEY=somethingx
    #DATABASE_URL=postgresql://localhost/autocompeter
    ALLOWED_HOSTS=localhost
    ES_CONNECTIONS_URLS=elasticsearch:9200
    AUTH0_CLIENT_SECRET="optional"

Simply run:

    docker-compose build
    docker-compose up

And now you should have a server running on `http://localhost:8000`


And to run the tests with Docker:

    docker-compose run web /usr/local/bin/python manage.py test

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
