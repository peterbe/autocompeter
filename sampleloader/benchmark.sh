#!/bin/bash -e

export URL="http://localhost:3000"

echo "Bulk loading"
./populate.py -d 8 --destination=$URL --domain="mydomain" --bulk
echo "The home page"
wrk -c 10 -d5 -t5 $URL | grep Requests
echo "Single word search 'p'"
wrk -c 10 -d5 -t5 "$URL/v1?q=p&d=mydomain" | grep Requests
echo "Single word search 'python'"
wrk -c 10 -d5 -t5 "$URL/v1?q=python&d=mydomain" | grep Requests
echo "Single word search 'xxxxxx'"
wrk -c 10 -d5 -t5 "$URL/v1?q=xxxxx&d=mydomain" | grep Requests
echo "Double word search 'python', 'te'"
wrk -c 10 -d5 -t5 "$URL/v1?q=python%20te&d=mydomain" | grep Requests
echo "Double word search 'xxxxxxxx', 'yyyyyyy'"
wrk -c 10 -d5 -t5 "$URL/v1?q=xxxxxxxx%20yyyyyy&d=mydomain" | grep Requests
