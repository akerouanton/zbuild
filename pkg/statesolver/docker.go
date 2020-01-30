package statesolver

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/containerd/containerd/remotes"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
	// RootDir is the path to the root of the build context.
	RootDir       string
	ImageResolver remotes.Resolver
}

func (s DockerSolver) ExecImage(
	ctx context.Context,
	imageRef string,
	cmd []string,
) (*bytes.Buffer, error) {
	strcmd := strings.Join(cmd, "; ")
	shellCmd := []string{
		"/bin/sh", "-o", "errexit",
		"-c", strcmd,
	}

	err := s.pullImage(ctx, imageRef)
	if err != nil {
		return nil, xerrors.Errorf(
			"failed to execute %q in %q from %s: %w", strcmd, imageRef, err)
	}

	c, err := s.createContainer(ctx, imageRef, shellCmd)
	if err != nil {
		return nil, err
	}
	defer s.removeContainer(ctx, c)

	err = s.startContainerAndWait(ctx, c)
	outbuf, _, readErr := s.fetchContainerLogs(ctx, c, true, false)
	if err != nil {
		return outbuf, xerrors.Errorf("failed to execute cmd %q in image %q: %w",
			strcmd, imageRef, err)
	}
	return outbuf, readErr
}

func (s DockerSolver) startContainerAndWait(ctx context.Context, containerID string) error {
	waitch, errch := s.Client.ContainerWait(ctx, containerID,
		container.WaitConditionNextExit)

	err := s.Client.ContainerStart(ctx, containerID,
		types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	select {
	case msg := <-waitch:
		if msg.Error != nil {
			return xerrors.Errorf("ContainerWait failed: %s", msg.Error.Message)
		}
		if msg.StatusCode != 0 {
			return xerrors.Errorf("command exited with code %d", msg.StatusCode)
		}
	case err := <-errch:
		return err
	}

	return nil
}

func (s DockerSolver) fetchContainerLogs(
	ctx context.Context,
	containerID string,
	stdout,
	stderr bool,
) (*bytes.Buffer, *bytes.Buffer, error) {
	var outbuf *bytes.Buffer
	var errbuf *bytes.Buffer

	opts := types.ContainerLogsOptions{
		ShowStdout: stdout,
		ShowStderr: stderr,
	}
	r, err := s.Client.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		err := xerrors.Errorf("failed to fetch logs for container %s: %w",
			containerID, err)
		return outbuf, errbuf, err
	}

	outbuf = &bytes.Buffer{}
	errbuf = &bytes.Buffer{}

	if _, err := stdcopy.StdCopy(outbuf, errbuf, r); err != nil {
		err := xerrors.Errorf("failed to read logs for container %s: %w",
			containerID, err)
		return outbuf, errbuf, err
	}

	return outbuf, errbuf, nil
}

func (s DockerSolver) FileExists(
	ctx context.Context,
	filepath string,
	source *builddef.Context,
) (bool, error) {
	if source == nil {
		return false, nil
	}

	// @TODO: implement a proper way to test if a file exists instead of reading it
	_, err := s.ReadFile(ctx, filepath, s.FromContext(source))
	found := err != FileNotFound

	if source.Type == builddef.ContextTypeGit {
		// @TODO: for now, ReadFile never returns FileNotFound when reading
		// from a Git context. As such, we can only consider that the given
		// file doesn't exist when it returns an error and thus we need to
		// silent this error.
		err = nil
	}

	return found, err
}

func (s DockerSolver) ReadFile(
	ctx context.Context,
	filepath string,
	opt ReadFileOpt,
) ([]byte, error) {
	return opt(ctx, filepath)
}

func (s DockerSolver) FromContext(c *builddef.Context, _ ...llb.LocalOption) ReadFileOpt {
	return func(ctx context.Context, filepath string) ([]byte, error) {
		if c == nil {
			return []byte{}, nil
		}

		switch c.Type {
		case builddef.ContextTypeLocal:
			return s.readFromLocalContext(filepath)
		case builddef.ContextTypeGit:
			return s.readFromGitContext(ctx, c, filepath)
		}

		return []byte{}, xerrors.Errorf(
			"context type %q is not supported", string(c.Type))
	}
}

func (s DockerSolver) readFromLocalContext(filepath string) ([]byte, error) {
	fullpath := path.Join(s.RootDir, filepath)
	raw, err := ioutil.ReadFile(fullpath)
	if os.IsNotExist(err) {
		return raw, xerrors.Errorf("failed to read %s from build context: %w", filepath, FileNotFound)
	} else if err != nil {
		return raw, xerrors.Errorf("failed to read %s from build context: %w", filepath, err)
	}

	return raw, nil
}

func (s DockerSolver) readFromGitContext(
	ctx context.Context,
	c *builddef.Context,
	filepath string,
) ([]byte, error) {
	repoURI := normalizeRepoURI(c)
	sourceRef := sourceRefOrHead(c)
	// git show fails when the filepath starts with a slash.
	filepath = strings.TrimPrefix(filepath, "/")

	outbuf, err := s.ExecImage(ctx, imageGit, []string{
		fmt.Sprintf("git clone --depth 1 %s /tmp/repo 1>/dev/null 2>&1", repoURI),
		"cd /tmp/repo",
		fmt.Sprintf("git show %s:%s", sourceRef, filepath)})
	if err != nil {
		return []byte{}, xerrors.Errorf(
			"failed to read file from git context: %w", err)
	}

	out := outbuf.Bytes()
	logrus.Debugf("Reading %s from git context: %s", filepath, string(out))

	return out, nil
}

func (s DockerSolver) FromImage(image string) ReadFileOpt {
	return func(ctx context.Context, filepath string) ([]byte, error) {
		var res []byte

		err := s.pullImage(ctx, image)
		if err != nil {
			return res, xerrors.Errorf("failed to read %s from %s: %w", filepath, image, err)
		}

		cid, err := s.createContainer(ctx, image, []string{})
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

	logrus.Debugf("Reading file %s from container %s", filepath, cid)

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
	// Don't pull agin if the image already exists
	if err == nil {
		return nil
	}
	if !client.IsErrNotFound(err) {
		return err
	}

	logrus.Debugf("Pulling %s", image)

	var r io.ReadCloser
	r, err = s.Client.ImagePull(ctx, image, types.ImagePullOptions{
		// @TODO: add support for authenticated registries/private images
		Platform: "amd64",
	})
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = ioutil.ReadAll(r)
	return err
}

func (s DockerSolver) createContainer(
	ctx context.Context,
	image string,
	cmd []string,
) (string, error) {
	logrus.Debugf("Creating container from image %s", image)

	cfg := container.Config{
		Image:  image,
		Cmd:    cmd,
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

func (s DockerSolver) ResolveImageRef(ctx context.Context, imageRef string) (string, error) {
	normalized, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return "", err
	}

	if canonical, ok := normalized.(reference.Canonical); ok {
		return canonical.String(), nil
	}

	_, desc, err := s.ImageResolver.Resolve(ctx, normalized.String())
	if err != nil {
		return "", err
	}

	resolved, err := reference.WithDigest(normalized, desc.Digest)
	if err != nil {
		return "", err
	}

	return resolved.String(), nil
}
