private.pem:
	openssl genrsa -out private.pem 4096

public.pem: private.pem
	openssl rsa -in private.pem -pubout -outform PEM -out public.pem

.PHONY: load-fixtures
load-fixtures:
	# api-platform/demo doesn't contain database migrations like a real project
	# would. Instead, we have to load fixtures but this requires some dev
	# dependencies. And since the php image has been built from a prod stage,
	# we have to intall dev deps first.
	docker-compose exec php /bin/sh -o errexit -c "\
		composer install --prefer-dist --no-scripts; \
		bin/console doctrine:schema:drop --full-database --force; \
		bin/console doctrine:schema:create; \
		bin/console hautelook:fixtures:load --append --env=dev"
