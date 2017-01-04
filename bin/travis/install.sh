#!/bin/bash
# pwd is the git repo.
set -e

echo "Upgrade pip & wheel"
pip install --quiet -U pip wheel

echo "Installing Python dependencies"
pip install --quiet --require-hashes -r requirements.txt

echo "Creating a test database"
psql -c 'create database autocompeter;' -U postgres
