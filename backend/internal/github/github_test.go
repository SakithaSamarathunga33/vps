package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	old := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = old })
	return NewClient("test-token")
}

func TestListOpenPullRequests(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widgets/pulls" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("state"); got != "open" {
			t.Fatalf("state = %q, want open", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":     1,
				"number": 7,
				"title":  "Add feature",
				"state":  "open",
				"user":   map[string]any{"login": "octocat"},
				"html_url": "https://github.com/acme/widgets/pull/7",
			},
		})
	})

	prs, err := client.ListOpenPullRequests("acme", "widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("len(prs) = %d, want 1", len(prs))
	}
	want := PullRequest{ExternalID: "1", Number: 7, Title: "Add feature", State: "open", Author: "octocat", URL: "https://github.com/acme/widgets/pull/7"}
	if prs[0] != want {
		t.Fatalf("prs[0] = %+v, want %+v", prs[0], want)
	}
}

func TestListOpenIssuesSkipsPullRequests(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 2, "number": 3, "title": "Bug", "state": "open", "html_url": "https://github.com/acme/widgets/issues/3"},
			{"id": 4, "number": 5, "title": "A PR, not an issue", "state": "open", "html_url": "https://github.com/acme/widgets/pull/5", "pull_request": map[string]any{}},
		})
	})

	issues, err := client.ListOpenIssues("acme", "widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1 (PR entry should be filtered out)", len(issues))
	}
	want := Issue{ExternalID: "2", Number: 3, Title: "Bug", State: "open", URL: "https://github.com/acme/widgets/issues/3"}
	if issues[0] != want {
		t.Fatalf("issues[0] = %+v, want %+v", issues[0], want)
	}
}

func TestGetLatestCommit(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"sha": "abc123def456", "commit": map[string]any{"message": "Fix bug\n\nLonger body"}},
		})
	})

	commit, err := client.GetLatestCommit("acme", "widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commit == nil {
		t.Fatal("commit = nil, want non-nil")
	}
	if commit.SHA != "abc123def456" || commit.Message != "Fix bug" {
		t.Fatalf("commit = %+v", commit)
	}
}

func TestGetLatestCommitReturnsNilForEmptyRepository(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Git Repository is empty."})
	})

	commit, err := client.GetLatestCommit("acme", "empty-repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commit != nil {
		t.Fatalf("commit = %+v, want nil", commit)
	}
}

func TestListOpenPullRequestsEmptyResult(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widgets/pulls" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("state"); got != "open" {
			t.Fatalf("state = %q, want open", got)
		}
		_ = json.NewEncoder(w).Encode([]any{})
	})

	prs, err := client.ListOpenPullRequests("acme", "widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prs == nil {
		t.Fatal("prs = nil, want non-nil empty slice")
	}
	if len(prs) != 0 {
		t.Fatalf("len(prs) = %d, want 0", len(prs))
	}
}

func TestListOpenIssuesEmptyResult(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{})
	})

	issues, err := client.ListOpenIssues("acme", "widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues == nil {
		t.Fatal("issues = nil, want non-nil empty slice")
	}
	if len(issues) != 0 {
		t.Fatalf("len(issues) = %d, want 0", len(issues))
	}
}

func TestGetLatestWorkflowRun(t *testing.T) {
	client := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"workflow_runs": []map[string]any{
				{"status": "completed", "conclusion": "success"},
			},
		})
	})

	run, err := client.GetLatestWorkflowRun("acme", "widgets")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run == nil || run.Status != "completed" || run.Conclusion != "success" {
		t.Fatalf("run = %+v", run)
	}
}
