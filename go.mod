module github.com/NiR-/zbuild

go 1.12

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/NiR-/notpecl v0.0.0-20200330122501-5974f6f2e95b
	github.com/buildkite/interpolate v0.0.0-20181028012610-973457fa2b4c
	github.com/containerd/containerd v1.5.4
	github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
	github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c
	github.com/go-test/deep v1.0.6
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.3
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/mcuadros/go-version v0.0.0-20190830083331-035f6764e8d2
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452
	github.com/mitchellh/mapstructure v1.3.3
	github.com/moby/buildkit v0.7.2
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v0.0.7
	github.com/tonistiigi/fsutil v0.0.0-20200326231323-c2c7d7b0e144
	github.com/twpayne/go-vfs v1.4.2
	golang.org/x/xerrors v0.0.0-20191011141410-1b5146add898
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305

replace github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200227195959-4d242818bf55

replace github.com/docker/docker => github.com/docker/docker v1.4.2-0.20200227233006-38f52c9fec82

replace github.com/snyh/go-dpkg-parser => github.com/NiR-/go-dpkg-parser v0.0.0-20190908004503-d7a2aa288b

replace github.com/docker/docker v1.14.0-0.20190319215453-e7b5f7dbe98c => github.com/docker/docker v1.4.2-0.20190319215453-e7b5f7dbe98c

replace golang.org/x/crypto v0.0.0-20190129210102-0709b304e793 => golang.org/x/crypto v0.0.0-20180904163835-0709b304e793
