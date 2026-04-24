package clicmd

import (
	"context"
	"fmt"
	"io"

	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/versioning"
)

type YankDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

type YankArgs struct {
	Slug    string // @owner/slug@v1.2.3
	Version string // overrides the version embedded in Slug
}

// splitVersion accepts "@owner/slug@v1.2.3" and returns (@owner/slug, v1.2.3).
func splitVersion(slug string) (string, string) {
	// Find the last '@'. If it's the leading '@', there's no version.
	last := -1
	for i := range slug {
		if slug[i] == '@' {
			last = i
		}
	}
	if last > 0 {
		return slug[:last], slug[last+1:]
	}
	return slug, ""
}

func resolveOwnerSlugVersion(slug, explicitVersion string) (owner, s, ver string, err error) {
	base, embedded := splitVersion(slug)
	n, err := namespace.Parse(base)
	if err != nil {
		return "", "", "", err
	}
	if n.IsBare {
		return "", "", "", fmt.Errorf("owner required: use @owner/slug")
	}
	v := explicitVersion
	if v == "" {
		v = embedded
	}
	if v != "" {
		if _, err := versioning.ParseVersion(v); err != nil {
			return "", "", "", fmt.Errorf("invalid version %q: %w", v, err)
		}
	}
	return n.Owner, n.Slug, v, nil
}

// Yank calls POST /api/tools/@owner/slug/@version/yank.
func Yank(_ context.Context, deps YankDeps, args YankArgs) error {
	if err := AttachSavedToken(deps.API); err != nil {
		return err
	}
	owner, slug, ver, err := resolveOwnerSlugVersion(args.Slug, args.Version)
	if err != nil {
		return err
	}
	if ver == "" {
		return fmt.Errorf("yank requires a version (e.g. @foo/bar@v1.2.3)")
	}
	if err := deps.API.Yank(owner, slug, ver); err != nil {
		return err
	}
	fmt.Fprintf(deps.Stdout, "Yanked @%s/%s@%s\n", owner, slug, ver)
	return nil
}

type DeprecateDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

type DeprecateArgs struct {
	Slug    string // @owner/slug (optionally @version)
	Message string
	Version string
}

func Deprecate(_ context.Context, deps DeprecateDeps, args DeprecateArgs) error {
	if err := AttachSavedToken(deps.API); err != nil {
		return err
	}
	owner, slug, ver, err := resolveOwnerSlugVersion(args.Slug, args.Version)
	if err != nil {
		return err
	}
	if args.Message == "" {
		return fmt.Errorf("deprecate: --message required")
	}
	if err := deps.API.Deprecate(owner, slug, ver, args.Message); err != nil {
		return err
	}
	if ver == "" {
		fmt.Fprintf(deps.Stdout, "Deprecated @%s/%s (whole package)\n", owner, slug)
	} else {
		fmt.Fprintf(deps.Stdout, "Deprecated @%s/%s@%s\n", owner, slug, ver)
	}
	return nil
}
