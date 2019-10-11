// Package client is highly inspired by https://github.com/genuinetools/img/tree/master/client
package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/NiR-/webdf/pkg/builder"
	"github.com/containerd/containerd/snapshots/native"
	"github.com/moby/buildkit/control"
	ociexecutor "github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/gateway/forwarder"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/filesync"
	"github.com/moby/buildkit/session/testutil"
	"github.com/moby/buildkit/solver/bboltcachestorage"
	"github.com/moby/buildkit/util/network/netproviders"
	"github.com/moby/buildkit/worker"
	"github.com/moby/buildkit/worker/base"
	"github.com/moby/buildkit/worker/runc"
	"golang.org/x/xerrors"
)

const (
	frontendName = "webdf.v0"
	sessionName  = "poc-buildkit-daemonless"
)

// Client holds the information for the client we will use for communicating
// with the buildkit controller.
type Client struct {
	builder builder.Builder
	backend string
	root    string

	sessionManager *session.Manager
	controller     *control.Controller
}

// NewClient returns a new client for communicating with the buildkit controller.
// It takes the path where buildkit state dirs will be create as argument.
func NewClient(b builder.Builder, root string) (*Client, error) {
	// Create the start of the client.
	return &Client{
		builder: b,
		backend: "native",
		root:    root,
	}, nil
}

// SessionManager returns the existing Buildkit session manager instance and
// creates one first if it doesn't exist yet.
func (c *Client) SessionManager() (*session.Manager, error) {
	if c.sessionManager == nil {
		var err error
		c.sessionManager, err = session.NewManager()
		if err != nil {
			return nil, err
		}
	}
	return c.sessionManager, nil
}

// Session starts a new build session with access to local registry credentials
// and to local directories needed for that build session.
func (c *Client) Session(
	ctx context.Context,
	localDirs map[string]string,
) (*session.Session, session.Dialer, error) {
	m, err := c.SessionManager()
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to create session manager: %v", err)
	}
	s, err := session.NewSession(ctx, sessionName, "")
	if err != nil {
		return nil, nil, xerrors.Errorf("failed to create session: %v", err)
	}
	syncedDirs := make([]filesync.SyncedDir, 0, len(localDirs))
	for name, d := range localDirs {
		syncedDirs = append(syncedDirs, filesync.SyncedDir{Name: name, Dir: d})
	}
	s.Allow(filesync.NewFSSyncProvider(syncedDirs))
	s.Allow(authprovider.NewDockerAuthProvider(os.Stdout))
	return s, sessionDialer(s, m), err
}

func sessionDialer(s *session.Session, m *session.Manager) session.Dialer {
	// @TODO: use grpchijacker?
	return session.Dialer(testutil.TestStream(testutil.Handler(m.HandleConn)))
}

// CreateController takes care of creating a new build controller. This is the
// part of Builkit usually run through buildkitd. This method sets the builder 
// configured on the client as the unique Buildkit frontend supported by the
// controller.
func (c *Client) CreateController() error {
	// As Builtkit has been designed to run multiple builds in parallel, it needs
	// a session manager to associate build sessions and grpc connections.
	sm, err := c.SessionManager()
	if err != nil {
		return xerrors.Errorf("failed to create session manager: %v", err)
	}

	// As Buildkit has been designed to support multiple platforms, it has a
	// concept of workers which are associated with specific platforms. Thus we
	// need to create a Worker Controller with a single worker initialized on the current host.
	w, err := newWorker(c.root)
	if err != nil {
		return xerrors.Errorf("failed to create buildkit worker: %v", err)
	}

	wc := &worker.Controller{}
	if err := wc.Add(w); err != nil {
		return xerrors.Errorf("failed to add buildkit worker to controller: %v", err)
	}
	if err != nil {
		return xerrors.Errorf("failed to create worker controller: %v", err)
	}

	// Add the frontends.
	frontends := map[string]frontend.Frontend{}
	frontends[frontendName] = forwarder.NewGatewayForwarder(wc, c.builder.Build)

	// Create the cache storage
	cacheStorage, err := bboltcachestorage.NewStore(filepath.Join(c.root, "cache.db"))
	if err != nil {
		return err
	}

	// Create the controller.
	controller, err := control.NewController(control.Opt{
		SessionManager:   sm,
		WorkerController: wc,
		Frontends:        frontends,
		CacheKeyStorage:  cacheStorage,
		// @TODO: add cache importers/exporters
	})
	if err != nil {
		return fmt.Errorf("creating new controller failed: %v", err)
	}

	// Set the controller for the client.
	c.controller = controller

	return nil
}

func newWorker(rootDir string) (*base.Worker, error) {
	snFactory := runc.SnapshotterFactory{
		Name: "native",
		New:  native.NewSnapshotter,
	}
	labels := map[string]string{}
	networkOpt := netproviders.Opt{
		Mode: "host",
	}
	dnsCfg := &ociexecutor.DNSConfig{}

	workerOpt, err := runc.NewWorkerOpt(rootDir, snFactory, true, ociexecutor.ProcessSandbox, labels, nil, networkOpt, dnsCfg)
	if err != nil {
		return nil, err
	}

	return base.NewWorker(workerOpt)
}
