module github.com/NiR-/webdf

go 1.12

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/NiR-/notpecl v0.0.0-20191203181030-776d9e829a9e
	github.com/bbuck/go-lexer v0.0.0-20150530081543-8752f4c00663 // indirect
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c
	github.com/go-test/deep v1.0.4
	github.com/golang/mock v1.1.1
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/hashicorp/go-version v1.2.0
	github.com/mitchellh/mapstructure v1.1.2
	github.com/moby/buildkit v0.6.2-0.20191002152821-f7042823e340
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opencontainers/image-spec v1.0.1
	github.com/sirupsen/logrus v1.4.2
	github.com/snyh/go-dpkg-parser v0.0.0-20171208093826-d45a4679150f
	github.com/spf13/cobra v0.0.5
	github.com/tonistiigi/fsutil v0.0.0-20190819224149-3d2716dd0a4d
	golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/snyh/go-dpkg-parser => github.com/NiR-/go-dpkg-parser v0.0.0-20190908004503-d7a2aa288b
