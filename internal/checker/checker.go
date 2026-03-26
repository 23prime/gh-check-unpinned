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

// RESTClient is the interface for making GitHub REST API calls.
type RESTClient interface {
	Get(path string, response any) error
}

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

// Checker performs unpinned action checks against GitHub repositories.
type Checker struct {
	client RESTClient
}

// New creates a new Checker with the given REST client.
func New(client RESTClient) *Checker {
	return &Checker{client: client}
}

// ListRepos returns repositories under the given owner (org or user).
func (c *Checker) ListRepos(owner string) ([]RepoInfo, error) {
	repos, err := c.listRepoPages("orgs/%s/repos", owner)
	if err != nil {
		var httpErr *api.HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusNotFound {
			return nil, err
		}
		return c.listRepoPages("users/%s/repos", owner)
	}
	return repos, nil
}

func (c *Checker) listRepoPages(pathFmt, owner string) ([]RepoInfo, error) {
	var all []RepoInfo
	for page := 1; ; page++ {
		var pageRepos []RepoInfo
		path := fmt.Sprintf(pathFmt+"?per_page=100&page=%d", owner, page)
		if err := c.client.Get(path, &pageRepos); err != nil {
			return nil, err
		}
		all = append(all, pageRepos...)
		if len(pageRepos) < 100 {
			break
		}
	}
	return all, nil
}

// CheckRepo returns unpinned action references found in the repository's workflows.
func (c *Checker) CheckRepo(owner, repo string) ([]string, error) {
	var entries []contentEntry
	err := c.client.Get(fmt.Sprintf("repos/%s/%s/contents/.github/workflows", owner, repo), &entries)
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
		unpinned, err := c.findUnpinned(owner, repo, e.Path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Path, err)
		}
		for _, u := range unpinned {
			results = append(results, fmt.Sprintf("%s/%s/%s: %s", owner, repo, e.Path, u))
		}
	}
	return results, nil
}

func (c *Checker) findUnpinned(owner, repo, path string) ([]string, error) {
	var fc fileContent
	if err := c.client.Get(fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, path), &fc); err != nil {
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
