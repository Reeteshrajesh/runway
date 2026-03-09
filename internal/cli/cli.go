// Package cli implements the runway command-line interface.
//
// All os.Exit calls happen exclusively in cmd/runway/main.go.
// cli.Run returns errors; it never calls os.Exit itself.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Run is the CLI entry point. It receives os.Args[1:] and the build version.
func Run(args []string, version string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "deploy":
		return runDeploy(rest)
	case "rollback":
		return runRollback(rest)
	case "status":
		return runStatus(rest)
	case "releases":
		return runReleases(rest)
	case "listen":
		return runListen(rest)
	case "log":
		return runLog(rest)
	case "version", "--version", "-v":
		fmt.Printf("runway %s\n", version)
		return nil
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		printUsage(os.Stderr)
		return errors.New("unknown command")
	}
}

// baseDir returns the runway working directory.
// Checks GITOPS_DIR env var first, falls back to /opt/runway.
func baseDir() string {
	if d := os.Getenv("GITOPS_DIR"); d != "" {
		return strings.TrimRight(d, "/")
	}
	return "/opt/runway"
}

// repoURL returns the git repository URL from GITOPS_REPO env var.
func repoURL() string {
	return os.Getenv("GITOPS_REPO")
}

// gitToken returns the optional git auth token from GITOPS_GIT_TOKEN env var.
func gitToken() string {
	return os.Getenv("GITOPS_GIT_TOKEN")
}

// requireArg returns an error if the args slice has fewer than n elements.
func requireArg(args []string, n int, usage string) error {
	if len(args) < n {
		return fmt.Errorf("usage: %s", usage)
	}
	return nil
}

// newFlagSet returns a FlagSet configured with the standard error handling.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `runway — lightweight git-based deployment manager

Usage:
  runway <command> [flags]

Commands:
  deploy <commit>             Deploy a specific commit
  rollback <commit>           Roll back to a previously deployed commit
  status                      Show current deployment status
  releases                    List all stored releases
  listen                      Start the webhook listener (HTTP server)
  log <commit>                Print the deploy log for a commit
  version                     Print version information

Environment variables:
  GITOPS_DIR          Working directory (default: /opt/runway)
  GITOPS_REPO         Git repository URL (required for deploy)
  GITOPS_GIT_TOKEN    Git HTTPS auth token (optional)

Run 'runway <command> --help' for command-specific flags.`)
}
