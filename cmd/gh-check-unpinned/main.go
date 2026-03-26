package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/23prime/gh-check-unpinned/internal/checker"
	"github.com/cli/go-gh/v2/pkg/api"
)

func main() {
	includeArchived := flag.Bool("include-archived", false, "Include archived repositories")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: gh check-unpinned [--include-archived] <owner>")
		os.Exit(1)
	}
	owner := args[0]

	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ch := checker.New(client)

	repos, err := ch.ListRepos(owner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to list repos for %q: %v\n", owner, err)
		os.Exit(1)
	}

	foundAny := false
	checkedAny := false
	for _, r := range repos {
		if r.Archived && !*includeArchived {
			continue
		}
		checkedAny = true
		findings, err := ch.CheckRepo(owner, r.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s/%s: %v\n", owner, r.Name, err)
			continue
		}
		for _, f := range findings {
			fmt.Println(f)
			foundAny = true
		}
	}

	if !checkedAny {
		fmt.Println("No repositories checked (all repositories are archived; use --include-archived to include them).")
	} else if !foundAny {
		fmt.Println("All actions are SHA-pinned.")
	}
}
