package statesolver

import (
	"archive/tar"
	"context"
	"io"
	"io/ioutil"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

type DockerSolver struct {
	Client *client.Client
	Labels map[string]string
}

func (f DockerSolver) FetchFile(ctx context.Context, image, filepath string) ([]byte, error) {
	var res []byte

	err := f.pullImage(ctx, image)
	if err != nil {
		return res, err
	}

	cid, err := f.createContainer(ctx, image)
	if err != nil {
		return res, err
	}
	defer f.removeContainer(ctx, cid)

	r, _, err := f.Client.CopyFromContainer(ctx, cid, filepath)
	if err != nil {
		if client.IsErrNotFound(err) {
			return res, xerrors.Errorf("path %q not found in %q", filepath, image)
		}
		return res, err
	}
	defer r.Close()

	expectedName := path.Base(filepath)
	tarR := tar.NewReader(r)
	h := new(tar.Header)

	for h.Name != expectedName {
		var err error
		h, err = tarR.Next()
		if err == io.EOF {
			return res, xerrors.Errorf("path %q could not be found", filepath)
		}
		if err != nil {
			return res, err
		}
	}
	if h.Typeflag == tar.TypeLink || h.Typeflag == tar.TypeSymlink {
		newpath := h.Linkname
		if !path.IsAbs(h.Linkname) {
			newpath = path.Join(path.Dir(filepath), h.Linkname)
		}
		return f.FetchFile(ctx, image, newpath)
	}
	if h.Typeflag != tar.TypeReg {
		return res, xerrors.Errorf("could not fetch path %q, it's not a regular file", filepath)
	}
	return ioutil.ReadAll(tarR)
}

func (f DockerSolver) pullImage(ctx context.Context, image string) error {
	_, _, err := f.Client.ImageInspectWithRaw(ctx, image)
	if client.IsErrNotFound(err) {
		var r io.ReadCloser
		r, err = f.Client.ImagePull(ctx, image, types.ImagePullOptions{
			// @TODO: add support for authenticated registries/private images
			Platform: "amd64",
		})
		if err == nil {
			defer r.Close()
			_, err = ioutil.ReadAll(r)
		}
	}
	return err
}

func (f DockerSolver) createContainer(ctx context.Context, image string) (string, error) {
	cfg := container.Config{
		Image:  image,
		Cmd:    []string{},
		Labels: f.Labels,
	}
	hostCfg := container.HostConfig{}
	networkCfg := network.NetworkingConfig{}
	resp, err := f.Client.ContainerCreate(ctx, &cfg, &hostCfg, &networkCfg, "")
	if err != nil {
		return "", xerrors.Errorf("could not create container: %w", err)
	}
	return resp.ID, nil
}

func (f DockerSolver) removeContainer(ctx context.Context, ID string) {
	err := f.Client.ContainerRemove(ctx, ID, types.ContainerRemoveOptions{})
	if err != nil {
		logrus.Error(err)
	}
}
