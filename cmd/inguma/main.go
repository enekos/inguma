// Command inguma is the user-facing CLI for the inguma marketplace.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/adapters/all"
	"github.com/enekos/inguma/internal/apiclient"
	"github.com/enekos/inguma/internal/clicmd"
	"github.com/enekos/inguma/internal/state"
)

// defaultAPI is the production marketplace URL. Override with --api.
const defaultAPI = "https://inguma.dev"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the testable seam.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	sub := args[0]
	rest := args[1:]

	// Global flags live on each subcommand's FlagSet so -h works naturally.
	ctx := context.Background()

	switch sub {
	case "install":
		return runInstall(ctx, rest, stdout, stderr)
	case "uninstall":
		return runUninstall(ctx, rest, stdout, stderr)
	case "list":
		return runList(rest, stdout, stderr)
	case "search":
		return runSearch(ctx, rest, stdout, stderr)
	case "show":
		return runShow(ctx, rest, stdout, stderr)
	case "doctor":
		return runDoctor(rest, stdout, stderr)
	case "upgrade":
		return runUpgrade(ctx, rest, stdout, stderr)
	case "publish":
		return runPublish(ctx, rest, stdout, stderr)
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "inguma: unknown command %q\n\n", sub)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: inguma <command> [flags]

Commands:
  install    Install a tool into detected harnesses
  uninstall  Remove a tool
  list       Show installed tools
  search     Search the marketplace
  show       Show a tool's details and install snippets
  doctor     Report harness detection status
  upgrade    Upgrade lockfile-pinned packages to newest patch/minor version
  publish    Tag, push, and poll ingestion of an inguma tool

Run "inguma <command> -h" for command-specific flags.
`)
}

func parseHarnesses(csv string) []string {
	if csv == "" {
		return nil
	}
	return strings.Split(csv, ",")
}

func runInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	harness := fs.String("harness", "", "comma-separated harness IDs (default: all detected)")
	dryRun := fs.Bool("dry-run", false, "print the diff without applying")
	yes := fs.Bool("y", false, "skip confirmation")
	rangeSpec := fs.String("range", "", "semver range, e.g. ^1.2 (versioned slugs only)")
	lockDir := fs.String("lock-dir", "", "directory containing inguma.lock (default: cwd; use - to disable)")
	frozen := fs.Bool("frozen", false, "refuse to resolve anything not pinned in inguma.lock")
	slugArg := ""
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		slugArg = fs.Arg(0)
	}
	if !*frozen && slugArg == "" {
		fmt.Fprintln(stderr, "install: slug required (or pass --frozen to install every lockfile entry)")
		return 2
	}
	err := clicmd.Install(ctx, clicmd.InstallDeps{
		API:       apiclient.New(*apiURL),
		Adapters:  all.Default(),
		StatePath: state.DefaultPath(),
		Stdout:    stdout,
	}, clicmd.InstallArgs{
		Slug:      slugArg,
		Harnesses: parseHarnesses(*harness),
		DryRun:    *dryRun,
		AssumeYes: *yes,
		RangeSpec: *rangeSpec,
		LockDir:   *lockDir,
		Frozen:    *frozen,
	})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runUninstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	harness := fs.String("harness", "", "restrict to harness IDs")
	yes := fs.Bool("y", false, "skip confirmation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "uninstall: slug required")
		return 2
	}
	err := clicmd.Uninstall(ctx, clicmd.UninstallDeps{
		Adapters:  all.Default(),
		StatePath: state.DefaultPath(),
		Stdout:    stdout,
	}, clicmd.UninstallArgs{
		Slug:      fs.Arg(0),
		Harnesses: parseHarnesses(*harness),
		AssumeYes: *yes,
	})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := clicmd.List(clicmd.ListDeps{StatePath: state.DefaultPath(), Stdout: stdout}); err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runSearch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	kind := fs.String("kind", "", "filter: mcp or cli")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	q := strings.Join(fs.Args(), " ")
	if q == "" {
		fmt.Fprintln(stderr, "search: query required")
		return 2
	}
	err := clicmd.Search(ctx, clicmd.SearchDeps{API: apiclient.New(*apiURL), Stdout: stdout}, clicmd.SearchArgs{Query: q, Kind: *kind})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runShow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "show: slug required")
		return 2
	}
	err := clicmd.Show(ctx, clicmd.ShowDeps{API: apiclient.New(*apiURL), Stdout: stdout}, clicmd.ShowArgs{Slug: fs.Arg(0)})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runUpgrade(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	harness := fs.String("harness", "", "comma-separated harness IDs (default: all detected)")
	dryRun := fs.Bool("dry-run", false, "print the diff without applying")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	slug := ""
	if fs.NArg() > 0 {
		slug = fs.Arg(0)
	}
	err := clicmd.Upgrade(ctx, clicmd.UpgradeDeps{
		API:       apiclient.New(*apiURL),
		Adapters:  all.Default(),
		StatePath: state.DefaultPath(),
		Stdout:    stdout,
	}, clicmd.UpgradeArgs{
		Slug:      slug,
		Harnesses: parseHarnesses(*harness),
		DryRun:    *dryRun,
	})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := clicmd.Doctor(clicmd.DoctorDeps{Adapters: all.Default(), Stdout: stdout}); err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}

func runPublish(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	fs.SetOutput(stderr)
	apiURL := fs.String("api", defaultAPI, "marketplace API URL")
	repo := fs.String("repo", "", "path to tool repo (default: current directory)")
	remote := fs.String("remote", "origin", "git remote to push the tag to")
	timeout := fs.Duration("timeout", 10*time.Minute, "how long to poll for ingestion (default: 10m)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	err := clicmd.Publish(ctx, clicmd.PublishDeps{
		API:    apiclient.New(*apiURL),
		Stdout: stdout,
	}, clicmd.PublishArgs{
		RepoDir: *repo,
		Remote:  *remote,
		Timeout: *timeout,
	})
	if err != nil {
		fmt.Fprintln(stderr, "inguma:", err)
		return 1
	}
	return 0
}
