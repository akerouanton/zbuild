.PHONY: load-fixtures
load-fixtures:
	# symfony/demo doesn't contain database migrations like a real project
	# would. Instead, we have to load fixtures but this requires some dev
	# dependencies. And since the php image has been built from a prod stage,
	# we have to intall dev deps first.
	docker-compose exec php /bin/bash -o errexit -c "\
		composer install --prefer-dist --no-scripts; \
		bin/console doctrine:schema:drop --full-database --force || true; \
		bin/console doctrine:schema:create; \
		bin/console doctrine:fixtures:load --append --env=dev"

.PHONY: test
test: load-fixtures
	curl --silent --fail http://localhost/en/blog/ >/dev/null
