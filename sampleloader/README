This is just some scripts I use to load in some sample data on the
autocompeter.com home page.


How I load in all blog posts for local development:

    ./populate.py --flush -d 8 --destination="http://localhost:3000" --domain="localhost:3000" --bulk


Then to check it run:

    curl "http://localhost:3000/v1?d=localhost:3000&q=trag"


How I load all the movies:

    ./populate.py -d 8 --destination="http://localhost:3000" --domain="movies-2014" --dataset=movies-2014.json --bulk

How to check it:

    curl "http://localhost:3000/v1?d=movies-2014&q=the"
