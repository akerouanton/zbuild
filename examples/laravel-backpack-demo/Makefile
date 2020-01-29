.PHONY: init
init:
	# Laravel-Buildpack/Demo has DB migrations but these don't create fake data
	# in the DB. Thus, we need to to load fixtures after applying the
	# migrations, but this requires some dev dependencies. Because the php
	# image has been built from a prod stage, we have to intall dev deps first.
	docker-compose exec php /bin/sh -o errexit -c "\
		composer install --prefer-dist --no-scripts; \
		php artisan key:generate --force; \
		php artisan cache:clear; \
		php artisan migrate --force; \
		php artisan db:seed --force"
