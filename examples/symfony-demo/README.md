# Symfony Demo

##### Prerequisites

To build this example you need:

* Docker v18.09+ with experimental features enabled ;
* docker-compose ;
* [`jq`] and [`yq`] (used by the `build` script) ;

##### Run this example

```bash
$ cd examples/symfony-demo

# Unfortunately, docker-compose isn't able to build images using Buildkit yet.
# As such, we have to rely on following script to parse docker-compose.yml
# file and build images.
$ ../build

# Finally, you can start the demo...
$ docker-compose -d
# And load the fixtures.
$ make load-fixtures
```

[`jq`]: https://stedolan.github.io/jq/download/
[`yq`]: https://github.com/kislyuk/yq
