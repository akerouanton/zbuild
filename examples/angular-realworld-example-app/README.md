# Angular example app

See https://github.com/gothinkster/angular-realworld-example-app.

##### Prerequisites

To build and run this example you need:

* Docker v18.09+ with experimental features enabled ;
* docker-compose ;
* [`jq`] and [`yq`] (used by the `build` script) ;

##### Run this example

```bash
$ cd examples/php/angular-realworld-example-app

# Unfortunately, docker-compose isn't able to build images using Buildkit yet.
# As such, we have to rely on following script to parse docker-compose.yml
# file and build images.
$ ../build

# Finally, you can start the demo
$ docker-compose -d
```

You can then browse to http://localhost/.


[`jq`]: https://stedolan.github.io/jq/download/
[`yq`]: https://github.com/kislyuk/yq
