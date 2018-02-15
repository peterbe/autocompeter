.PHONY: build clean migrate shell currentshell stop test run django-shell docs psql

help:
	@echo "Welcome to the django-peterbe\n"
	@echo "The list of commands for local development:\n"
	@echo "  build            Builds the docker images for the docker-compose setup"
	@echo "  ci               Run the test with the CI specific Docker setup"
	@echo "  clean            Stops and removes all docker containers"
	@echo "  migrate          Runs the Django database migrations"
	@echo "  shell            Opens a Bash shell"
	@echo "  currentshell     Opens a Bash shell into existing running 'web' container"
	@echo "  test             Runs the Python test suite"
	@echo "  run              Runs the whole stack, served on http://localhost:8000/"
	@echo "  stop             Stops the docker containers"
	@echo "  django-shell     Django integrative shell"
	@echo "  psql             Open the psql cli"


clean: stop
	docker-compose rm -f
	rm -fr .docker-build

migrate:
	docker-compose run web python manage.py migrate --run-syncdb

shell:
	# Use `-u 0` to automatically become root in the shell
	docker-compose run --user 0 web bash

currentshell:
	# Use `-u 0` to automatically become root in the shell
	docker-compose exec --user 0 web bash

psql:
	docker-compose run db psql -h db -U postgres

stop:
	docker-compose stop

test:
	@bin/test.sh

run:
	docker-compose up web

django-shell:
	docker-compose run web python manage.py shell

make-index:
	docker-compose run web python manage.py create-index
	# docker-compose run web /usr/local/bin/python manage.py create-index
