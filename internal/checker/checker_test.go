package checker_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/23prime/gh-check-unpinned/internal/checker"
	"github.com/cli/go-gh/v2/pkg/api"
)

// mockClient implements RESTClient for testing.
type mockClient struct {
	responses map[string]any
	errors    map[string]error
}

func (m *mockClient) Get(path string, resp any) error {
	if err, ok := m.errors[path]; ok {
		return err
	}
	if data, ok := m.responses[path]; ok {
		b, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, resp)
	}
	return &api.HTTPError{StatusCode: http.StatusNotFound}
}

func newMock(responses map[string]any, errors map[string]error) *mockClient {
	if errors == nil {
		errors = map[string]error{}
	}
	return &mockClient{responses: responses, errors: errors}
}

func encodeWorkflow(yaml string) string {
	return base64.StdEncoding.EncodeToString([]byte(yaml))
}

// --- ListRepos ---

func TestListRepos_Org(t *testing.T) {
	mock := newMock(map[string]any{
		"orgs/myorg/repos?per_page=100&page=1": []map[string]any{
			{"name": "repo-a", "archived": false, "fork": false},
			{"name": "repo-b", "archived": true, "fork": false},
			{"name": "repo-c", "archived": false, "fork": true},
		},
	}, nil)

	repos, err := checker.New(mock).ListRepos("myorg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}
	if repos[1].Archived != true {
		t.Errorf("expected repo-b to be archived")
	}
	if repos[2].Fork != true {
		t.Errorf("expected repo-c to be a fork")
	}
}

func TestListRepos_UserFallback(t *testing.T) {
	mock := newMock(map[string]any{
		"users/myuser/repos?per_page=100&page=1": []map[string]any{
			{"name": "repo-a", "archived": false},
		},
	}, map[string]error{
		"orgs/myuser/repos?per_page=100&page=1": &api.HTTPError{StatusCode: http.StatusNotFound},
	})

	repos, err := checker.New(mock).ListRepos("myuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "repo-a" {
		t.Errorf("unexpected repos: %v", repos)
	}
}

// --- CheckRepo ---

func TestCheckRepo_NoWorkflowsDir(t *testing.T) {
	mock := newMock(nil, nil) // all paths return 404

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestCheckRepo_UnpinnedActions(t *testing.T) {
	wf := encodeWorkflow(`
jobs:
  build:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@4d34df0c2316fe8122ab82dc22947d607c0c91f0
`)
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
		"repos/owner/repo/contents/.github/workflows/ci.yml": map[string]any{
			"content": wf,
		},
	}, nil)

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.Repo != "owner/repo" || f.Workflow != ".github/workflows/ci.yml" || f.Action != "actions/checkout@v4" {
		t.Errorf("unexpected finding: %+v", f)
	}
}

func TestCheckRepo_AllPinned(t *testing.T) {
	wf := encodeWorkflow(`
jobs:
  build:
    steps:
      - uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683
      - uses: actions/setup-go@4d34df0c2316fe8122ab82dc22947d607c0c91f0
`)
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
		"repos/owner/repo/contents/.github/workflows/ci.yml": map[string]any{
			"content": wf,
		},
	}, nil)

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestCheckRepo_SkipsLocalActions(t *testing.T) {
	wf := encodeWorkflow(`
jobs:
  build:
    steps:
      - uses: ./local-action
`)
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
		"repos/owner/repo/contents/.github/workflows/ci.yml": map[string]any{
			"content": wf,
		},
	}, nil)

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for local action, got %v", findings)
	}
}

func TestCheckRepo_ReusableWorkflow(t *testing.T) {
	wf := encodeWorkflow(`
jobs:
  call:
    uses: owner/repo/.github/workflows/reusable.yml@main
`)
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
		"repos/owner/repo/contents/.github/workflows/ci.yml": map[string]any{
			"content": wf,
		},
	}, nil)

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unpinned reusable workflow, got %d", len(findings))
	}
}

func TestCheckRepo_SkipsDirectoryEntries(t *testing.T) {
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "subdir", "path": ".github/workflows/subdir", "type": "dir"},
		},
	}, nil)

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for directory entry, got %v", findings)
	}
}

func TestCheckRepo_SkipsNonYAMLFiles(t *testing.T) {
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "README.md", "path": ".github/workflows/README.md", "type": "file"},
		},
	}, nil)

	findings, err := checker.New(mock).CheckRepo("owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-YAML file, got %v", findings)
	}
}

func TestCheckRepo_WorkflowsDirError(t *testing.T) {
	serverErr := &api.HTTPError{StatusCode: http.StatusInternalServerError}
	mock := newMock(nil, map[string]error{
		"repos/owner/repo/contents/.github/workflows": serverErr,
	})

	_, err := checker.New(mock).CheckRepo("owner", "repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckRepo_FileContentError(t *testing.T) {
	fetchErr := errors.New("network error")
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
	}, map[string]error{
		"repos/owner/repo/contents/.github/workflows/ci.yml": fetchErr,
	})

	_, err := checker.New(mock).CheckRepo("owner", "repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckRepo_InvalidBase64(t *testing.T) {
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
		"repos/owner/repo/contents/.github/workflows/ci.yml": map[string]any{
			"content": "!!!not-valid-base64!!!",
		},
	}, nil)

	_, err := checker.New(mock).CheckRepo("owner", "repo")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestCheckRepo_InvalidYAML(t *testing.T) {
	mock := newMock(map[string]any{
		"repos/owner/repo/contents/.github/workflows": []map[string]any{
			{"name": "ci.yml", "path": ".github/workflows/ci.yml", "type": "file"},
		},
		"repos/owner/repo/contents/.github/workflows/ci.yml": map[string]any{
			"content": encodeWorkflow("jobs: [invalid: yaml"),
		},
	}, nil)

	_, err := checker.New(mock).CheckRepo("owner", "repo")
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestListRepos_Pagination(t *testing.T) {
	page1 := make([]map[string]any, 100)
	for i := range page1 {
		page1[i] = map[string]any{"name": fmt.Sprintf("repo-%d", i), "archived": false}
	}
	mock := newMock(map[string]any{
		"orgs/myorg/repos?per_page=100&page=1": page1,
		"orgs/myorg/repos?per_page=100&page=2": []map[string]any{
			{"name": "repo-100", "archived": false},
		},
	}, nil)

	repos, err := checker.New(mock).ListRepos("myorg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repos) != 101 {
		t.Errorf("expected 101 repos across 2 pages, got %d", len(repos))
	}
}

func TestListRepos_BothEndpointsFail(t *testing.T) {
	apiErr := &api.HTTPError{StatusCode: http.StatusUnauthorized}
	mock := newMock(nil, map[string]error{
		"orgs/owner/repos?per_page=100&page=1":  apiErr,
		"users/owner/repos?per_page=100&page=1": apiErr,
	})

	_, err := checker.New(mock).ListRepos("owner")
	if err == nil {
		t.Fatal("expected error when both endpoints fail, got nil")
	}
}
