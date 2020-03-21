# Express.js example app

See https://github.com/gothinkster/node-express-realworld-example-app.

##### Prerequisites

To build and run this example you need:

* Docker v18.09+ with experimental features enabled ;
* docker-compose ;
* [`jq`] and [`yq`] (used by the `build` script) ;

##### Run this example

```bash
$ cd examples/php/expressjs-realworld-example-app

# Unfortunately, docker-compose isn't able to build images using Buildkit yet.
# As such, we have to rely on following script to parse docker-compose.yml
# file and build images.
$ ../build

# Finally, you can start the demo
$ docker-compose -d
```

You can now fetch the list of articles from the API:

```bash
$ curl http://localhost/api/articles
```

[`jq`]: https://stedolan.github.io/jq/download/
[`yq`]: https://github.com/kislyuk/yq
