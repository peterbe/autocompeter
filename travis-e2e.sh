#!/bin/bash

go build server.go authkeys.go api.go
./server -port=3000 -redisDatabase=8 &
# I wish a I knew a way to install python things without sudo
sudo pip install nose redis requests
nosetests
