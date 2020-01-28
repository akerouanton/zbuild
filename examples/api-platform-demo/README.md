# API Platform Demo

##### Prerequisites

To build and run this example you need:

* Docker v18.09+ with experimental features enabled ;
* docker-compose ;
* [`jq`] and [`yq`] (used by the `build` script) ;

##### Run this example

```bash
$ cd examples/php/api-platform-demo

# Unfortunately, docker-compose isn't able to build images using Buildkit yet.
# As such, we have to rely on following script to parse docker-compose.yml
# file and build images.
$ ../build

# Before starting the freshly built containers, you need to create the pair of
# RSA keys used to sign JWT
$ make public.pem

# You can then start the demo
$ docker-compose -d

# Finally, you need to load data fixtures
$ make load-fixtures
```

[`jq`]: https://stedolan.github.io/jq/download/
[`yq`]: https://github.com/kislyuk/yq
