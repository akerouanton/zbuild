package image

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/client/llb/imagemetaresolver"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// Image represents a OCI compliant image with extra fields for Docker
// (e.g. Healthcheck).
type Image struct {
	specs.Image

	Config ImageConfig `json:"config,omitempty"`
}

// ImageConfig represents a OCI compliant image config with extra fields for
// Docker (e.g. Healthcheck).
type ImageConfig struct {
	specs.ImageConfig

	Healthcheck *HealthConfig `json:",omitempty"`
}

// HealthConfig represents the healthcheck configuration used by Docker. It comes from:
// https://github.com/moby/buildkit/blob/2b2bdac1b84b33dcac99211c0a0f0b50c93e0e8f/frontend/dockerfile/dockerfile2llb/image.go#L12
type HealthConfig struct {
	// Test is the test to perform to check that the container is healthy.
	// An empty slice means to inherit the default.
	// The options are:
	// {} : inherit healthcheck
	// {"NONE"} : disable healthcheck
	// {"CMD", args...} : exec arguments directly
	// {"CMD-SHELL", command} : run command with system's default shell
	Test []string `json:",omitempty"`

	// Zero means to inherit. Durations are expressed as integer nanoseconds.
	Interval    time.Duration `json:",omitempty"` // Interval is the time to wait between checks.
	Timeout     time.Duration `json:",omitempty"` // Timeout is the time to wait before considering the check to have hung.
	StartPeriod time.Duration `json:",omitempty"` // The start period for the container to initialize before the retries starts to count down.

	// Retries is the number of consecutive failures needed to consider a container as unhealthy.
	// Zero means inherit.
	Retries int `json:",omitempty"`
}

// LoadMeta looks for image metadata for the given imageRef. It returns an Image
// when metadata could be found and an error otherwise.
func LoadMeta(ctx context.Context, imageRef string) (*Image, error) {
	_, meta, err := imagemetaresolver.Default().ResolveImageConfig(ctx, imageRef, llb.ResolveImageConfigOpt{})
	if err != nil {
		return nil, err
	}

	var img Image
	if err := json.Unmarshal(meta, &img); err != nil {
		return nil, err
	}

	return &img, nil
}

// CloneMeta does a deep copy of the given Image and returns the copied Image.
func CloneMeta(src *Image) *Image {
	img := src
	img.Config = src.Config
	img.Config.Env = append([]string{}, src.Config.Env...)
	img.Config.Cmd = append([]string{}, src.Config.Cmd...)
	img.Config.Entrypoint = append([]string{}, src.Config.Entrypoint...)
	if img.Config.Labels == nil {
		img.Config.Labels = map[string]string{}
	}
	if img.Config.Volumes == nil {
		img.Config.Volumes = map[string]struct{}{}
	}

	return img
}
