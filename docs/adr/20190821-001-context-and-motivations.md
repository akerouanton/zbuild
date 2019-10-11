# ADR 001: General context & motivations

This ADR presents the general context and motivations for finding a new way to
solve DX issues related to the day-to-day use of Docker.

* Date: 2019-08-21
* Author(s): Albin Kerouanton
* Status: In review

## Context

#### Docker and Dockerfiles

Docker rose in the recent years and is now widely used by the industry.
It offers a generic way of packaging and shipping code and can be leveraged to
ensure all of a project environments (e.g. dev, staging, prod) are kept as
close of each others as possible. This very specific feature also makes it easy
to configure and ship third-party softwares on all environments, including
other developers' computer, even if they have no prior knowledge of these
softwares.

As such, Docker helps avoiding "it works on my machine" syndrome and can
effectively reconcile both ops and devs people by making them work on a single
set of files, both following same practices like code review, CI/CD process,
etc...

In modst cases, it also moved the line between ops and devs responsibility:
for instance, a PHP API can't live without a php-fpm and a HTTP server, thus
some teams started to integrate php and nginx config files and their inner
working as part of their core skills. It sounds like a reasonable take and
should actually be encouraged.

However, despite its apparent easyness, using Docker and even more, writting
Dockerfiles to set up everything needed for a given project has nothing to do
with either core dev skills nor with mastering 3rd-party softwares. It's mostly
cumbersome, repetitive and thus annoying system stuff, even for ops people. As
such, most developers seem to write Dockerfiles that just work and don't care
much about good/best practices and other details (e.g. image layer ordering,
whitelist-based .dockerignore, bind-mounted config files in dev env, etc...).

Let alone writing and building Dockerfiles, developers joining a project still
have to know exactly how containers have been set up to decide what and when
docker commands should be run (e.g. dc restart vs dc build+up vs some custom
command emptying a specific queue when a worker is restarted).

When developers have to set up or maintain Dockerfiles on multiple projects,
it seems the process becomes soon repetitive and they tend to reuse existing
templates to not lose time. This is an apparent indicator of the lack of
reusability and might be seen as an obstacle to spreading good/best practices.

As examples of this repetitive complexity: php:7.0 images include a broken 
version of `tail` binary that has to be replaced with a newer version coming
from `debian:testing` to be able to properly tail from a named pipe written by
Monolog because that version of FPM badly forwards logs. Another example:
PHP 7.4 don't include pecl anymore, thus any team using XDebug will have to
sort this mess before using it again. And as a last example: developers wanting
to set healthcheck on PHP images will probably face issues with cgi-fcgi binary
and DNS resolution.

First, everyone in the industry have eventually to deal with above issues and
second, all the steps needed to overcome these issues don't add any value
*per se*, only the end result might (e.g. finding a secure way of installing
pecl vs shipping redis extension to have better perf).

Moreover, Docker alone is not enough to set up deployment pipelines, be it 
executed manually or automatically. It generally gets executed through some task
runner like make, fabric, ansible, etc... and once again things tend to be
copy/pasted across projects. This is not a problem with Docker itself as it
follows UNIX philosophy but rather the sign we're lacking some higher-level 
tool(s) to help us industrialize this part of the development workflow.

*Note: for the sake of this ADR, template GH repos are considered
copy/pasting as it generally involves the same maintainance efforts.*

*Note: this ADR mostly talks about PHP projects but the same reasoning apply to
any other (Web) language/framework. Fortunately, the steps needed to build
images for Python-based or Rails-based or PHP-based APIs are the same, only
tools and language differs.*

#### Compose

Most of the time when Docker is used for dev purposes, it's done through
docker-compose as it allows to easily tie containers together. Compose exposes
a 1-to-1 binding with docker CLI tool. It makes it a great, generic tool but 
once again it's quite low-level and thus let user deal with a lot of details. 
For instance, developers generally want one leader and one worker node whenever
they scale their DB service to 2 replicas.

#### Buildkit

Docker build engine has been extracted and rewritten into github.com/moby/buildkit.
This new build engine has better caching and speed promises, and offers some 
new, advanced Dockerfile features: Git-based build context, secret and ssh
bind-mouting in Dockerfiles, etc... (the complete list is available
[here](https://github.com/moby/buildkit/blob/master/frontend/dockerfile/docs/experimental.md)).

These features are great as they open new doors, especially regarding SSH keys 
and secret management as this has always been a pain and regarded as a bad
practice.

There's another new feature, one that might be of importance for us: Buildkit
offers a way for anyone to provide their own Dockerfile syntax and the
conversion process between this format and low-level instructions. The
conversion process for Dockerfiles has even been rewritten using this new
mechanism.

All of this is available in Buildkit itself: it has a server and a client
component like Docker. It's able to build/push/inspect images.

It's also available in `docker` CLI tool through `docker buildx` subcommands.
However, it needs experimental features to be enabled first. And this is a pain
point: as this is not generally available on CIs right now, we're still stuck.

Both tools now support rootless build/execution. That is: their daemon could be
run by any non-system user (like uid 1000). This is particularly interesting in
the context of CI execution: currently it's hard (if not
impossible) to securely run Docker in Docker without privileged containers.
However, CIs using containers (like CircleCI or Github Actions) don't give
privileged containers (for security reasons). So building images is not so easy.

### DX issues

At this point, we can identify following problems:

1. Writing, maintaining and debugging Dockerfiles is not easy whereas the
expected end result is always the same (e.g. a nginx server with php-fpm,
for a PHP API) ;
2. Reusing pieces of Dockerfiles/config files across projects is a common
practice but is mostly copy/pasting ;
3. Managing `.env` files and checking out new dist versions is hard;
4. Managing secrets in all environments is not an easy task ;
5. CI integration is not always easy as Docker have to run with privileged
rights ;
6. Spreading and enforcing good/best-practices (linters, security scanners,
etc..) is neither easy ;
7. Knowing what command to run and when needs some prior knowledge of the 
Dockerfile/Compose files structure. As a corollary, devs have too much 
docker commands to type on a day-to-day basis ;
8. Managing multiple environments (e.g. dev, staging, prod) is not so easy ;

To sum it up it looks like people would benefit from using PaaS-like,
declarative UI instead of the low-level UIs of Docker/Compose. Something that 
would give them control on important things (fpm or http config) but hide 
unimportant and boring low-level stuff.

## Alternatives

* https://github.com/dunglas/symfony-docker
* https://jolicode.com/blog/introducing-our-docker-starter-kit
* https://github.com/wodby/docker4wordpress

## Solutions

#### I. Derive Dockerfiles from Composer/NPM/... files (answers: 1, 2, 6)

Two PoCs around this idea have been written: 1. one in the form of a composer
plugin and 2. a second one in the form of a Python script. Both PoCs were pretty
much successful: we can effectively extract the platform requirements from
composer.lock (or during composer install/update commands), guess the libs
needed by platform reqs and thus generate working Dockerfiles based on
templates.

However generated files have the same down side than copy/pasted files: they
tend to be hard to maintain, especially when generated/pasted code have been 
updated to match specific requirements and even with automation tools.

One important issue was not covered by these PoCs: system libs versions have 
to be recorded somewhere to ensure build reproducibility. Also, extensions
can be extracted from composer.lock, but this file contains only version
constraints but no locked versions. Again, these have to be recorded somewhere.

#### II. Watch command (answers: 7)

This is a direct response to *issue 7*: devs, when working on their project,
should have to type the least amount of Docker-related commands. As such, we
can think of a `watch` command that would infer action(s) to take based on FS
notifications.

A PoC for this idea has already been written (https://github.com/KnpLabs/kit, private repo).
However, this PoC was using a file-format with a basic/low-level semantic:

```yml
infer: true

watchers:
  - path: src/
    action: restart
    # Here, services match the one defined by docker-compose files
    services: [php, cron]

  - path: web/uploads/
    action: none
```

#### III. Use custom LLB-based format (answers: 1, 2, 6)

This new feature could be leveraged to create our own build process and reuse 
inference idea proposed in *solution I*. Extensibility would be provided
through predefined extension points instead of a free-to-edit templatized file.
Thus, this would get rid of the maintainance issue associated with *solution I*.

However, this solution come with the same issue regarding reproducibility as
with templated Dockerfiles: system libs and extensions versions have to be
recorded somewhere. Moreover, with this solution, extension points have to be
set in some way, probably through a dedicated config file. Such file would look
something like this (following [these structures](https://github.com/NiR-/pocllb/blob/master/builder/config.go#L7-L32)):

```yml
# syntax=akerouanton/pocllb
build_config:
  version: 7.3
  fpm: true
  system_packages:
    some-specific-package: *
  extensions:
    redis: 5.0.2
    soap: *
  composer_dump:
    apcu: false
    classmap_authoritative: true
  source_dirs:
    - ./config
    - ./src
  extra_scripts:
    - ./bin/console
    - ./public/index.php
  integrations:
    blackfire: true
  external_files:
    - url: 
      compressed: true
      pattern: "blackfire-*.so"
      destination: 
      checksum: sha256:...
      owner: 1000
      mode: 0750
  stateful_dirs:
    - ./public/uploads
  healthcheck: true
```

Following *solution I* and usuability objectives, extensions would be infered
from composer.lock, system packages would be infered from extensions, composer 
dump flags would have sane defaults set (for prod stage) and that specific
blackfire external file would be actually included by blackfire integration
automatically. Ideally, source dirs, extra scripts and stateful dirs could be 
infered from framework versions or could be infered based on a dedicated
parameter.

#### IV. Use rootless build tool (answers: 5)

Docker now supports rootless execution/builds (and Buildkit too) but it's still
an experimental feature and it might not be generally available before some
releases (minor versions of the CLI tool being released every 6 months).
Moreover, it's not limited to build but impacts the whole execution of Docker
(run, available storage drivers, etc..).

* [`img`](https://github.com/genuinetools/img), 
* [`builkit`](https://github.com/moby/buildkit)

#### V. Provide generic commands to manage environments (partially answers: 4 and answers: 8)

See [this example Makefile](example.mk) to see what's generally implemented.

* Makefile are not that great: portability issue, limited/poor UX, yet another
language to learn.
* Using a language-specific tool is not the best: Python developers might not
want to learn PHP to use a specific task runner, and vice-versa.

#### VI. Docker CLI plugin

Proposal for CLI plugins: https://github.com/docker/cli/issues/1534

## Accepted solution

* Use a single binary answering all of the above issues with dedicated
subcommands.
* Use a lock file to record system libs and extensions versions
* Use a custom build syntax
* Implement custom build subcommand to run it in rootless mode and manage
version lock file
* Keep full compatibility with Docker and other Buildkit-based tools:
  * Syntax providers should be able to run with any Buildkit-based tool, but
obviously version locks can't be managed that way (but does not seem needed as 
this would probably be managed during development or CI runs, where designed
tool will probably be)
  * Parse designed tool config files from within syntax container
* Add a watch subcommand that leverage build definition to determine watcher
actions
* Despite having no language restrictions, for now it seems easier to create
LLB builders in Go, as there's already a package available for that.
