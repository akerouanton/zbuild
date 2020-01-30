# Laravel Backpack Demo

https://github.com/Laravel-Backpack/Demo

##### Prerequisites

To build and run this example you need:

* Docker v18.09+ with experimental features enabled ;
* docker-compose ;
* [`jq`] and [`yq`] (used by the `build` script) ;

##### Run this example

```bash
$ cd examples/php/laravel-backpack-demo

# Unfortunately, docker-compose isn't able to build images using Buildkit yet.
# As such, we have to rely on following script to parse docker-compose.yml
# file and build images.
$ ../build

# Then, you can start the demo.
$ docker-compose -d

# Finally, you need to initialize the APP_KEY and load the migrations and
# fixtures.
$ make init
```

[`jq`]: https://stedolan.github.io/jq/download/
[`yq`]: https://github.com/kislyuk/yq
