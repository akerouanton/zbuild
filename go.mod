module github.com/NiR-/webdf

go 1.12

require (
	github.com/NiR-/go-dpkg-parser v0.0.0-20190907233358-d7a2aa288b8b
	github.com/bbuck/go-lexer v0.0.0-20150530081543-8752f4c00663 // indirect
	github.com/containerd/console v0.0.0-20181022165439-0650fd9eeb50
	github.com/containerd/containerd v1.3.0-0.20190507210959-7c1e88399ec0
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c
	github.com/go-test/deep v1.0.3
	github.com/gogo/protobuf v1.2.0
	github.com/golang/mock v1.1.1
	github.com/golang/protobuf v1.2.0
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/json-iterator/go v1.1.8 // indirect
	github.com/mitchellh/mapstructure v1.1.2
	github.com/moby/buildkit v0.6.1
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runc v1.0.0-rc8
	github.com/pkg/errors v0.8.1
	github.com/rootless-containers/rootlesskit v0.6.0
	github.com/sirupsen/logrus v1.3.0
	github.com/snyh/go-dpkg-parser v0.0.0-20171208093826-d45a4679150f
	github.com/spf13/cobra v0.0.5
	github.com/theckman/go-flock v0.7.1 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20190327153851-3bbb99cdbd76
	go.etcd.io/etcd v3.3.17+incompatible
	golang.org/x/sync v0.0.0-20180314180146-1d60e4601c6f
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
	google.golang.org/grpc v1.20.1
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/snyh/go-dpkg-parser => github.com/NiR-/go-dpkg-parser v0.0.0-20190908004503-d7a2aa288b
