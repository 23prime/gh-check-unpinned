package checker

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"gopkg.in/yaml.v3"
)

// RepoInfo holds basic repository information.
type RepoInfo struct {
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type contentEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type fileContent struct {
	Content string `json:"content"`
}

type workflowFile struct {
	Jobs map[string]jobDef `yaml:"jobs"`
}

type jobDef struct {
	Uses  string    `yaml:"uses"`
	Steps []stepDef `yaml:"steps"`
}

type stepDef struct {
	Uses string `yaml:"uses"`
}

var shaRe = regexp.MustCompile(`@[0-9a-f]{40}$`)

// ListRepos returns repositories under the given owner (org or user).
func ListRepos(client *api.RESTClient, owner string) ([]RepoInfo, error) {
	var repos []RepoInfo
	if err := client.Get(fmt.Sprintf("orgs/%s/repos?per_page=100", owner), &repos); err == nil {
		return repos, nil
	}
	if err := client.Get(fmt.Sprintf("users/%s/repos?per_page=100", owner), &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

// CheckRepo returns unpinned action references found in the repository's workflows.
func CheckRepo(client *api.RESTClient, owner, repo string) ([]string, error) {
	var entries []contentEntry
	err := client.Get(fmt.Sprintf("repos/%s/%s/contents/.github/workflows", owner, repo), &entries)
	if err != nil {
		var httpErr *api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	var results []string
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		if !strings.HasSuffix(e.Name, ".yml") && !strings.HasSuffix(e.Name, ".yaml") {
			continue
		}
		unpinned, err := findUnpinned(client, owner, repo, e.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Path, err)
		}
		for _, u := range unpinned {
			results = append(results, fmt.Sprintf("%s/%s/%s: %s", owner, repo, e.Path, u))
		}
	}
	return results, nil
}

func findUnpinned(client *api.RESTClient, owner, repo, path string) ([]string, error) {
	var fc fileContent
	if err := client.Get(fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path), &fc); err != nil {
		return nil, err
	}

	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(fc.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	var wf workflowFile
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	var unpinned []string
	for _, j := range wf.Jobs {
		if u := j.Uses; u != "" && !isLocal(u) && !isSHAPinned(u) {
			unpinned = append(unpinned, u)
		}
		for _, s := range j.Steps {
			if u := s.Uses; u != "" && !isLocal(u) && !isSHAPinned(u) {
				unpinned = append(unpinned, u)
			}
		}
	}
	return unpinned, nil
}

func isLocal(uses string) bool {
	return strings.HasPrefix(uses, "./")
}

func isSHAPinned(uses string) bool {
	return shaRe.MatchString(uses)
}
