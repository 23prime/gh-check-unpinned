package main

import (
	"fmt"
	"os"

	"github.com/23prime/gh-check-unpinned/internal/checker"
	"github.com/cli/go-gh/v2/pkg/api"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: gh check-unpinned <owner>")
		os.Exit(1)
	}
	owner := os.Args[1]

	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	repos, err := checker.ListRepos(client, owner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to list repos for %q: %v\n", owner, err)
		os.Exit(1)
	}

	foundAny := false
	for _, r := range repos {
		findings, err := checker.CheckRepo(client, owner, r.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s/%s: %v\n", owner, r.Name, err)
			continue
		}
		for _, f := range findings {
			fmt.Println(f)
			foundAny = true
		}
	}

	if !foundAny {
		fmt.Println("All actions are SHA-pinned.")
	}
}
