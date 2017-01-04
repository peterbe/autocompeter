#!/bin/sh
pip install --quiet -U flake8
flake8 autocompeter --exclude=*/migrations/*

# should really eslint here too
