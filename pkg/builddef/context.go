package builddef

import (
	"net/url"
	"strings"

	"golang.org/x/xerrors"
)

type ContextType string

var (
	ContextTypeGit   = ContextType("git")
	ContextTypeLocal = ContextType("local")
)

func (ctype ContextType) IsValid() error {
	switch ctype {
	case ContextTypeGit:
		return nil
	case ContextTypeLocal:
		return nil
	}

	return xerrors.New("invalid context type: only \"local\" and \"git\" are supported")
}

// NewContext takes a source which can be either a string starting with
// "git://" followed by a repo URI or a local context name. It also takes an
// optional contextType that can be used to force the type of the context (no
// inference on the source format will be done).
func NewContext(source string, contextType string) (*Context, error) {
	if contextType == string(ContextTypeGit) ||
		strings.HasPrefix(source, "git://") {
		return newGitContext(source)
	}

	context := &Context{
		Source: source,
		Type:   ContextTypeLocal,
	}
	return context, nil
}

func newGitContext(sourceURL string) (*Context, error) {
	context := &Context{
		Type: ContextTypeGit,
	}

	u, err := url.Parse(sourceURL)
	if err != nil {
		return nil, err
	}

	refAndPath := strings.SplitN(u.Fragment, ":", 2)
	context.Reference = ""
	context.Path = ""

	if len(refAndPath[0]) > 0 {
		context.Reference = refAndPath[0]
	}
	if len(refAndPath) == 2 {
		context.Path = refAndPath[1]
	}

	u.Fragment = ""
	context.Source = u.String()

	return context, nil
}

type Context struct {
	GitContext `mapstructure:",squash"`

	Type ContextType
	// Source is either the name of the local context or the URI of the remote
	// context.
	Source string `mapstructure:"source"`
}

func (base *Context) Copy() *Context {
	if base == nil {
		return nil
	}

	return &Context{
		GitContext: base.GitContext,
		Type:       base.Type,
		Source:     base.Source,
	}
}

func (c *Context) IsValid() error {
	if c == nil {
		return nil
	}

	if err := c.Type.IsValid(); err != nil {
		return err
	}

	if c.Source == "" {
		return xerrors.New("invalid context: context source is empty")
	}

	return nil
}

func (c *Context) RawLocks() map[string]interface{} {
	return map[string]interface{}{
		"type":      c.Type,
		"source":    c.Source,
		"reference": c.Reference,
		"path":      c.Path,
	}
}

func (c *Context) IsGitContext() bool {
	return c != nil && c.Type == ContextTypeGit
}

func (c *Context) IsLocalContext() bool {
	return c != nil && c.Type == ContextTypeLocal
}

type GitContext struct {
	Reference string
	// Path contains the base root dir of the context in the git repo.
	Path string
}
