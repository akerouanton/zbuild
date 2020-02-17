package llbutils

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/NiR-/zbuild/pkg/builddef"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	"github.com/moby/buildkit/frontend/gateway/client"
	digest "github.com/opencontainers/go-digest"
	"golang.org/x/xerrors"
)

const (
	// APT is the const used to install apt packages with InstallSystemPackages.
	APT = "apt"
	APK = "apk"
)

var (
	// UnsupportedPackageManager is thrown when the package manager used by the
	// base image distro isn't supported by zbuild.
	UnsupportedPackageManager = xerrors.New("unspported package manager")
)

// SolveState takes any state and solve it. It returns the solve result, a
// unique ref and an error if any happens.
func SolveState(
	ctx context.Context,
	c client.Client,
	src llb.State,
) (*client.Result, client.Reference, error) {
	def, err := src.Marshal()
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to marshal LLB state: %w", err)
	}

	res, err := c.Solve(ctx, client.SolveRequest{
		Definition: def.ToPB(),
	})
	if err != nil {
		return nil, nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to get a single ref for source: %w", err)
	}

	return res, ref, nil
}

// ReadFile reads the content of the file at given filepath. It returns the
// file content, a bool indicating if the file was present and an error
// (excluding ErrNotExists).
func ReadFile(ctx context.Context, ref client.Reference, filepath string) ([]byte, bool, error) {
	content, err := ref.ReadFile(ctx, client.ReadRequest{
		Filename: filepath,
	})
	if err != nil && strings.Contains(err.Error(), "file does not exist") {
		return []byte{}, false, nil
	} else if err != nil {
		return []byte{}, false, err
	}

	return content, true, nil
}

func ImageSource(imageRef string, withMeta bool) llb.State {
	opts := []llb.ImageOption{}

	if withMeta {
		opts = append(opts, llb.WithMetaResolver(imagemetaresolver.Default()))
	}

	return llb.Image(imageRef, opts...)
}

func Copy(src llb.State, srcPath string, dest llb.State, destPath string, chown string) llb.State {
	copyOpts := []llb.CopyOption{
		&llb.CopyInfo{
			FollowSymlinks:      true,
			CopyDirContentsOnly: true,
			CreateDestPath:      true,
			AllowWildcard:       true,
		},
	}
	if chown != "" {
		copyOpts = append(copyOpts, llb.WithUser(chown))
	}

	return dest.File(
		llb.Copy(src, srcPath, destPath, copyOpts...),
		llb.WithCustomName(fmt.Sprintf("Copy %s", srcPath)))
}

func Shell(cmds ...string) llb.RunOption {
	cmd := strings.Join(cmds, "; ")
	cmd = strings.Replace(cmd, "\"", "\\\"", -1)
	return llb.Shlex("/bin/sh -o errexit -c \"" + cmd + "\"")
}

func Mkdir(state llb.State, owner string, dirs ...string) llb.State {
	for _, dir := range dirs {
		action := llb.Mkdir(dir, 0750,
			llb.WithParents(true),
			llb.WithUser(owner))
		state = state.File(action,
			llb.WithCustomName("Mkdir "+dir))
	}

	return state
}

// InstallSystemPackages installs the given packages with the given package
// manager. Packages map have to be a set of package names associated to their
// respective version.
func InstallSystemPackages(
	state llb.State,
	pkgMgr string,
	locks map[string]string,
) (llb.State, error) {
	if len(locks) == 0 {
		return state, nil
	}

	var cmds []string
	var pkgNames []string

	for pkgName := range locks {
		pkgNames = append(pkgNames, pkgName)
	}
	sort.Strings(pkgNames)

	packageSpecs := []string{}
	for _, pkgName := range pkgNames {
		packageSpecs = append(packageSpecs, pkgName+"="+locks[pkgName])
	}

	switch pkgMgr {
	case APT:
		cmds = []string{
			"apt-get update",
			fmt.Sprintf("apt-get install -y --no-install-recommends %s", strings.Join(packageSpecs, " ")),
			"rm -rf /var/lib/apt/lists/*",
		}
	case APK:
		cmds = []string{
			"apk add --no-cache " + strings.Join(packageSpecs, " "),
		}
	default:
		return llb.State{}, UnsupportedPackageManager
	}

	stepName := fmt.Sprintf("Install system packages (%s)", strings.Join(packageSpecs, ", "))
	state = state.Run(
		Shell(cmds...),
		llb.WithCustomName(stepName),
	).Root()

	return state, nil
}

// ExternalFile represents a file that should be loaded through HTTP at build-time.
type ExternalFile struct {
	URL         string
	Compressed  bool
	Pattern     string
	Destination string
	Checksum    string
	Mode        os.FileMode
	Owner       string
}

// CopyExternalFiles downloads the given list of ExternalFiles, each in their
// own DAG tree root (thus they're going to be executed in parallel),
// decompress and unpack them if required and finally copy them to the given
// state.
func CopyExternalFiles(state llb.State, externalFiles []ExternalFile) llb.State {
	for _, externalFile := range externalFiles {
		httpOpts := []llb.HTTPOption{
			llb.Filename("/out"),
			llb.WithCustomName("Download " + externalFile.URL),
		}

		if externalFile.Checksum != "" {
			httpOpts = append(httpOpts, llb.Checksum(digest.Digest(externalFile.Checksum)))
		}

		externalSource := llb.HTTP(externalFile.URL, httpOpts...)
		src := externalSource
		srcPath := "/out"
		adj := ""

		if externalFile.Compressed {
			decompressOpts := []llb.CopyOption{&llb.CopyInfo{
				AttemptUnpack: true,
			}}
			src = llb.Scratch().File(
				llb.Copy(src, "/out", "/decompressed", decompressOpts...),
				llb.WithCustomName("Decompress "+externalFile.URL))

			srcPath = "/decompressed"
			adj = "decompressed "
		}

		if externalFile.Pattern != "" {
			unpackOpts := []llb.CopyOption{&llb.CopyInfo{
				AttemptUnpack: true,
				AllowWildcard: true,
			}}
			unpackSrcPath := path.Join(srcPath, externalFile.Pattern)
			unpackAction := llb.Copy(src, unpackSrcPath, "/unpacked", unpackOpts...)

			src = llb.Scratch().File(unpackAction, llb.WithCustomName("Unpack "+externalFile.URL))
			srcPath = "/unpacked"
			adj = "unpacked "
		}

		copyInfo := &llb.CopyInfo{
			FollowSymlinks:      true,
			CopyDirContentsOnly: true,
			CreateDestPath:      true,
			AllowWildcard:       true,
		}
		if externalFile.Mode != 0 {
			copyInfo.Mode = &externalFile.Mode
		}

		copyOpts := []llb.CopyOption{copyInfo}
		if externalFile.Owner != "" {
			copyOpts = append(copyOpts, llb.WithUser(externalFile.Owner))
		}

		state = state.File(
			llb.Copy(src, srcPath, externalFile.Destination, copyOpts...),
			llb.WithCustomName(fmt.Sprintf("Copy %s%s to %s", adj, externalFile.URL, externalFile.Destination)))
	}

	return state
}

func FromContext(
	context *builddef.Context,
	opts ...llb.LocalOption,
) llb.State {
	switch context.Type {
	case builddef.ContextTypeGit:
		return llb.Git(context.Source, context.GitContext.Reference)
	case builddef.ContextTypeLocal:
		return llb.Local(context.Source, opts...)
	}

	panic(fmt.Sprintf("Unsupported context type %q", string(context.Type)))
}
