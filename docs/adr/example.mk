STAGE ?= dev
REMOTE ?= deploy@dsome.service.net
REMOTE_PATH=/home/deploy/$(STAGE)
IMAGE_TAG ?= dev
INTERACTIVE ?= $(shell test -t 0 && test "$${CI:-}" != "true" && echo 1)

SHELL=/bin/bash

ifeq ($(STAGE),prod)
	REMOTE=deploy@defimedoc.prod
endif

.PHONY: dev
dev: cp-dotenv init-networks generate-keys build install-deps start migrations

.PHONY: cp-dotenv
cp-dotenv:
	cp -n .env.dist .env

.PHONY: build
build: .ensure-stage-exists .validate-image-tag
	docker-compose -f docker-compose.$(STAGE).yml build $(SERVICES)

.PHONY: push
push: .ensure-stage-exists .validate-image-tag
	docker-compose -f docker-compose.$(STAGE).yml push

.PHONY: start
start:
	docker-compose -f docker-compose.$(STAGE).yml up -d --no-build --remove-orphans

.PHONY: start-proxy
start-proxy:
	export COMPOSE_PROJECT_NAME=defimedoc-proxy && \
		docker-compose -f docker-compose.reverse-proxy.$(STAGE).yml up -d

.PHONY: stop-proxy
stop-proxy:
	export COMPOSE_PROJECT_NAME=defimedoc-proxy && \
		docker-compose -f docker-compose.reverse-proxy.$(STAGE).yml down

.PHONY: install-deps
install-deps:
# Don't install deps if current is not dev, as there's no directory mounted in
# target containers and images for other stages don't have package managers.
ifeq ($(STAGE),dev)
	$(MAKE) install-deps-php install-deps-js
endif

.PHONY: install-deps-php
install-deps-php:
	docker-compose run --rm php composer install --prefer-dist

.PHONY: install-deps-js
install-deps-js:
	docker-compose run --rm admin yarn install
	docker-compose run --rm website yarn install

.PHONY: stop
stop:
	docker-compose down

.PHONY: init-networks
init-networks:
	-docker network create --internal --driver=bridge proxy_traefik

.PHONY: reset-database
reset-database:
	docker-compose exec php console doctrine:schema:drop --full-database --force
	docker-compose exec php console doctrine:migration:migrate --no-interaction

.PHONY: migrations
migrations: .wait-for-db
	docker-compose exec php console doctrine:migration:migrate --no-interaction

.PHONY: fixtures
fixtures:
	docker-compose run --rm php console hautelook:fixtures:load --env=dev --purge-with-truncate --no-interaction

.PHONY: fixtures-test
fixtures-test:
	docker-compose run --rm php console hautelook:fixtures:load --env=test --purge-with-truncate --no-interaction

.PHONY: generate-keys
generate-keys:
	if [ ! -f .ssh/public.pem ]; then apps/api/bin/ssh-keygen; chmod +r .ssh/private.pem; fi

#### Remote targets

.PHONY: remote-init
remote-init: .ensure-stage-exists .disallow-dev-env remote-init-networks
	-ssh -t ${REMOTE} 'mkdir -p ${REMOTE_PATH}/{apps/admin,apps/api,apps/website,docker/nginx}'
	-ssh -t ${REMOTE} 'mkdir /home/deploy/reverse-proxy'
	rsync --ignore-existing .env.dist ${REMOTE}:${REMOTE_PATH}/.env
	$(MAKE) remote-generate-keys remote-edit-env

.PHONY: remote-init-networks
remote-init-networks:
	-ssh -t ${REMOTE} 'docker network create --internal --driver=bridge proxy_traefik'

.PHONY: remote-generate-keys
remote-generate-keys:
	cat apps/api/bin/ssh-keygen | ssh -t ${REMOTE} '\
		cd ${REMOTE_PATH} && \
		if [ ! -f .ssh/public.pem ]; then /bin/bash /dev/stdin; fi'

.PHONY: remote-deploy
remote-deploy: .ensure-stage-exists .disallow-dev-env .validate-image-tag
	scp docker-compose.$(STAGE).yml ${REMOTE}:${REMOTE_PATH}/docker-compose.$(STAGE).yml
	ssh -t ${REMOTE} '\
		cd ${REMOTE_PATH} && \
		export IMAGE_TAG=$(IMAGE_TAG) && \
		docker-compose -f docker-compose.$(STAGE).yml pull --include-deps && \
		docker-compose -f docker-compose.$(STAGE).yml up -d --no-build --remove-orphans && \
		docker-compose ps'

.PHONY: remote-deploy-proxy
remote-deploy-proxy: .ensure-stage-exists .disallow-dev-env
ifeq ($(STAGE),preprod)
	scp docker-compose.reverse-proxy.preprod.yml ${REMOTE}:/home/deploy/reverse-proxy/docker-compose.yml
else ifeq ($(STAGE),prod)
	scp docker-compose.reverse-proxy.prod.yml ${REMOTE}:/home/deploy/reverse-proxy/docker-compose.yml
endif
	ssh -t ${REMOTE} '\
		cd /home/deploy/reverse-proxy/ && \
		export IMAGE_TAG=$(IMAGE_TAG) && \
		docker-compose pull --include-deps && \
		docker-compose up -d --no-build --remove-orphans && \
		docker-compose ps'

.PHONY: remote-edit-env
remote-edit-env: .ensure-stage-exists .disallow-dev-env
	ssh -t ${REMOTE} 'editor ${REMOTE_PATH}/.env'

.PHONY: remote-migrate
remote-migrate: .ensure-stage-exists .disallow-dev-env
	ssh -t ${REMOTE} '\
		cd ${REMOTE_PATH} && \
		docker-compose exec php console doctrine:migration:migrate --no-interaction'

.PHONY: remote-ps
remote-ps: .ensure-stage-exists .disallow-dev-env
	ssh -t ${REMOTE} '\
		cd ${REMOTE_PATH} && \
		docker-compose ps'

.PHONY: remote-logs
remote-logs: .ensure-stage-exists .disallow-dev-env
		ssh -t ${REMOTE} '\
			cd ${REMOTE_PATH} && \
			docker-compose logs -f $(SERVICES)'

.PHONY: remote-exec
remote-exec: .ensure-stage-exists .disallow-dev-env
	ssh -t ${REMOTE} '\
		cd ${REMOTE_PATH} && \
		docker-compose exec $(SERVICES) /bin/bash'

.PHONY: remote-shell
remote-shell:
	ssh -t ${REMOTE} '\
		cd ${REMOTE_PATH} && \
		/bin/bash -i'

#### Lint targets

.PHONY: lint-dockerfiles
lint-dockerfiles:
	@./bin/lint-dockerfiles

.PHONY: lint-compose-files
lint-compose-files:
	@for file in docker-compose.*.yml; do \
		docker-compose -f $$file config >/dev/null; \
	done

#### Internal targets - Preconditions

.PHONY: .ensure-stage-exists
.ensure-stage-exists:
ifeq (,$(wildcard docker-compose.$(STAGE).yml))
	@echo "Env $(STAGE) not supported."
	@exit 1
endif

.PHONY: .disallow-dev-env
.disallow-dev-env:
ifeq ($(STAGE),dev)
	@echo "You can't deploy dev stage to remote instance. It's designed to run locally only.\n"
	@exit 1
endif

.PHONY: .validate-image-tag
.validate-image-tag:
ifneq ($(STAGE),dev)
ifeq ($(IMAGE_TAG),)
	@echo "You can't build, push or deploy to prod without an IMAGE_TAG.\n"
	@exit 1
endif
endif

.PHONY: .wait-for-db
.wait-for-db:
	@./bin/wait-for-db
