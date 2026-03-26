package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/23prime/gh-check-unpinned/internal/checker"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/muesli/termenv"
)

var (
	stdout = termenv.NewOutput(os.Stdout)
	stderr = termenv.NewOutput(os.Stderr)
)

func main() {
	includeArchived := flag.Bool("include-archived", false, "Include archived repositories")
	jsonOutput := flag.Bool("json", false, "Output findings as JSON")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: gh check-unpinned [--include-archived] [--json] <owner>")
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

	var allFindings []checker.Finding
	checkedAny := false
	for _, r := range repos {
		if r.Archived && !*includeArchived {
			continue
		}
		checkedAny = true
		findings, err := ch.CheckRepo(owner, r.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, stderr.String(fmt.Sprintf("warn: %s/%s: %v", owner, r.Name, err)).
				Foreground(stderr.Color("3")).String())
			continue
		}
		allFindings = append(allFindings, findings...)
	}

	if *jsonOutput {
		out := allFindings
		if out == nil {
			out = []checker.Finding{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintln(os.Stderr, "error: failed to encode JSON:", err)
			os.Exit(1)
		}
		return
	}

	if !checkedAny {
		msg := "No repositories checked (all repositories are archived; use --include-archived to include them)."
		fmt.Println(stdout.String(msg).Foreground(stdout.Color("3")).String())
	} else if len(allFindings) == 0 {
		fmt.Println(stdout.String("All actions are SHA-pinned.").Foreground(stdout.Color("2")).String())
	} else {
		for _, f := range allFindings {
			fmt.Println(colorFinding(f))
		}
	}
}

// colorFinding colors a finding as "owner/repo/path: action@ref".
// The path prefix is rendered faint; the unpinned action is bold red.
func colorFinding(f checker.Finding) string {
	prefix := f.Repo + "/" + f.Workflow + ": "
	path := stdout.String(prefix).Faint().String()
	action := stdout.String(f.Action).Foreground(stdout.Color("9")).Bold().String()
	return path + action
}

