package statesolver

import (
	"archive/tar"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

// DockerSolver is the solver used by zbuild CLI tool. As such, it uses Docker
// to read files from images and resolve image references.
// @TODO: rename into LocalSolver?
type DockerSolver struct {
	Client *client.Client
	Labels map[string]string
}

func (s DockerSolver) ReadFile(
	ctx context.Context,
	filepath string,
	opt ReadFileOpt,
) ([]byte, error) {
	return opt(ctx, filepath)
}

func (s DockerSolver) FromBuildContext(opts ...llb.LocalOption) ReadFileOpt {
	return func(ctx context.Context, filepath string) ([]byte, error) {
		raw, err := ioutil.ReadFile(filepath)
		if os.IsNotExist(err) {
			return raw, xerrors.Errorf("failed to read %s from build context: %w", filepath, FileNotFound)
		} else if err != nil {
			return raw, xerrors.Errorf("failed to read %s from build context: %w", filepath, err)
		}
		return raw, nil
	}
}

func (s DockerSolver) FromImage(image string) ReadFileOpt {
	return func(ctx context.Context, filepath string) ([]byte, error) {
		var res []byte

		err := s.pullImage(ctx, image)
		if err != nil {
			return res, xerrors.Errorf("failed to read %s from %s: %w", filepath, image, err)
		}

		cid, err := s.createContainer(ctx, image)
		if err != nil {
			return res, xerrors.Errorf("failed to read %s from %s: %w", filepath, image, err)
		}
		defer s.removeContainer(ctx, cid)

		raw, err := s.readFromContainer(ctx, cid, filepath)
		if err != nil {
			return res, xerrors.Errorf("failed to read %s from %s: %w", filepath, image, err)
		}

		return raw, nil
	}
}

func (s DockerSolver) readFromContainer(
	ctx context.Context,
	cid string,
	filepath string,
) ([]byte, error) {
	var res []byte

	r, _, err := s.Client.CopyFromContainer(ctx, cid, filepath)
	if err != nil {
		if client.IsErrNotFound(err) {
			return res, FileNotFound
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
			return res, FileNotFound
		} else if err != nil {
			return res, err
		}
	}

	if h.Typeflag == tar.TypeLink || h.Typeflag == tar.TypeSymlink {
		newpath := h.Linkname
		if !path.IsAbs(h.Linkname) {
			newpath = path.Join(path.Dir(filepath), h.Linkname)
		}
		return s.readFromContainer(ctx, cid, newpath)
	}
	if h.Typeflag != tar.TypeReg {
		return res, xerrors.Errorf("could not fetch path %q, it's not a regular file", filepath)
	}

	return ioutil.ReadAll(tarR)
}

func (s DockerSolver) pullImage(ctx context.Context, image string) error {
	_, _, err := s.Client.ImageInspectWithRaw(ctx, image)
	if client.IsErrNotFound(err) {
		var r io.ReadCloser
		r, err = s.Client.ImagePull(ctx, image, types.ImagePullOptions{
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

func (s DockerSolver) createContainer(ctx context.Context, image string) (string, error) {
	cfg := container.Config{
		Image:  image,
		Cmd:    []string{},
		Labels: s.Labels,
	}
	hostCfg := container.HostConfig{}
	networkCfg := network.NetworkingConfig{}
	resp, err := s.Client.ContainerCreate(ctx, &cfg, &hostCfg, &networkCfg, "")
	if err != nil {
		return "", xerrors.Errorf("could not create container: %w", err)
	}
	return resp.ID, nil
}

func (s DockerSolver) removeContainer(ctx context.Context, ID string) {
	err := s.Client.ContainerRemove(ctx, ID, types.ContainerRemoveOptions{})
	if err != nil {
		logrus.Error(err)
	}
}
