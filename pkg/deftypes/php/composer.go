package php

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/NiR-/webdf/pkg/builddef"
	"github.com/NiR-/webdf/pkg/llbutils"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

// loadPlatformReqsFromContext loads composer.lock from build conext and adds
// any extensions declared there but not in webdf.yaml.
func loadPlatformReqsFromContext(
	ctx context.Context,
	c client.Client,
	stage *StageDefinition,
	opts builddef.BuildOpts,
) error {
	composerSrc := llb.Local("context",
		llb.IncludePatterns([]string{"composer.json", "composer.lock"}),
		llb.SessionID(opts.SessionID),
		llb.SharedKeyHint("composer-files"),
		llb.WithCustomName("load composer files from build context"))
	_, ref, err := llbutils.SolveState(ctx, c, composerSrc)
	if err != nil {
		return err
	}

	lockdata, ok, err := llbutils.ReadFile(ctx, ref, "composer.lock")
	if !ok {
		return nil
	}
	if err != nil {
		return xerrors.Errorf("could not load composer.lock: %v", err)
	}

	parsed, err := parsePlatformReqs(lockdata)
	if err != nil {
		return err
	}

	for ext, constraint := range parsed {
		if _, ok := stage.Extensions[ext]; !ok {
			stage.Extensions[ext] = constraint
		}
	}

	return nil
}

func loadPlatformReqsFromFS(stage *StageDefinition) error {
	lockdata, err := ioutil.ReadFile("composer.lock")
	if err != nil {
		logrus.Debugf("Could not load composer.lock: %+v", err)
		return nil
	}

	parsed, err := parsePlatformReqs(lockdata)
	if err != nil {
		return err
	}

	for ext, constraint := range parsed {
		if _, ok := stage.Extensions[ext]; !ok {
			stage.Extensions[ext] = constraint
		}
	}

	return nil
}

func parsePlatformReqs(lockdata []byte) (map[string]string, error) {
	var composerLock map[string]interface{}
	if err := json.Unmarshal(lockdata, &composerLock); err != nil {
		return map[string]string{}, xerrors.Errorf("could not unmarshal composer.lock: %v", err)
	}

	platformReqs, ok := composerLock["platform"]
	if !ok {
		return map[string]string{}, nil
	}

	exts := make(map[string]string, 0)
	for req, constraint := range platformReqs.(map[string]interface{}) {
		if !strings.HasPrefix(req, "ext-") {
			continue
		}

		ext := strings.TrimPrefix(req, "ext-")
		exts[ext] = constraint.(string)
	}

	return exts, nil
}
