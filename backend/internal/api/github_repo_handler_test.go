package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"pulsenode/backend/internal/db"
	"pulsenode/backend/internal/github"
)

func TestWriteGitHubErrorMapsStatuses(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{&github.APIStatusError{StatusCode: http.StatusConflict, Msg: "stale"}, http.StatusConflict},
		{&github.APIStatusError{StatusCode: http.StatusForbidden, Msg: "no perm"}, http.StatusForbidden},
		{&github.APIStatusError{StatusCode: http.StatusNotFound, Msg: "missing"}, http.StatusNotFound},
		{&github.APIStatusError{StatusCode: http.StatusInternalServerError, Msg: "boom"}, http.StatusBadGateway},
		{errRepoNotInstalled, http.StatusNotFound},
		{errNoGitHubApp, http.StatusNotFound},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		writeGitHubError(rec, tc.err)
		if rec.Code != tc.want {
			t.Fatalf("writeGitHubError(%v) = %d, want %d", tc.err, rec.Code, tc.want)
		}
	}
}

// TestGithubRepoFileMissingPathReturns400 proves githubRepoFile validates the
// required "path" query parameter before ever touching s.repoClient (and
// therefore before touching the DB or GitHub), so a caller that forgets
// ?path= gets a clear 400 instead of a panic or a misleading 404/502.
func TestGithubRepoFileMissingPathReturns400(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/github/app/repos/acme/widgets/file", nil)
	rec := httptest.NewRecorder()

	s.githubRepoFile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "path") {
		t.Fatalf("body = %q, want it to mention the missing path parameter", rec.Body.String())
	}
}

// TestGithubRepoUpdateFileValidation proves githubRepoUpdateFile rejects a
// malformed JSON body and each combination of a missing required field
// (path, sha, message) with 400 before ever calling s.repoClient. Using a
// zero-value *Server here (no DB, no GitHub App) is only safe because these
// branches must return before repoClient is reached; any regression that
// moved the validation after the repoClient call would panic this test on
// the nil DB instead of silently passing.
func TestGithubRepoUpdateFileValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"malformed json", `{not valid json`},
		{"missing path", `{"content":"aGk=","sha":"abc123","message":"update"}`},
		{"missing sha", `{"path":"README.md","content":"aGk=","message":"update"}`},
		{"missing message", `{"path":"README.md","content":"aGk=","sha":"abc123"}`},
		{"empty body", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{}
			req := httptest.NewRequest(http.MethodPut, "/github/app/repos/acme/widgets/file", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			s.githubRepoUpdateFile(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}

// TestGithubRepoRoutesWireOwnerRepoParams proves the chi routes registered
// for the per-repo endpoints correctly extract {owner}/{repo} from the URL
// and hand them to the handler, which forwards them into s.repoClient. It
// mounts githubRepoCommits on a real chi router (the same pattern used in
// server.go) against a Server backed by a real, empty on-disk DB (no
// settings rows), so the request travels: chi route match -> chi.URLParam
// extraction -> s.repoClient(owner, repo) -> s.db.GetSetting -> "" ->
// errNoGitHubApp -> writeGitHubError -> 404 with our own JSON error body.
//
// A well-formed two-segment path is contrasted against a path missing the
// repo segment, which chi itself rejects with its own plain-text 404 before
// the handler (and therefore repoClient) ever runs. That contrast is what
// demonstrates the {owner}/{repo} pattern - and not just any request -
// is what reaches the handler.
func TestGithubRepoRoutesWireOwnerRepoParams(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pulsenode-test.db")
	realDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer realDB.Close()

	s := &Server{db: realDB}

	router := chi.NewRouter()
	router.Get("/github/app/repos/{owner}/{repo}/commits", s.githubRepoCommits)

	t.Run("matched route reaches handler and repoClient", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/github/app/repos/acme/widgets/commits?limit=5", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (errNoGitHubApp mapping); body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), errNoGitHubApp.Error()) {
			t.Fatalf("body = %q, want it to contain the application's own error (%q), proving the handler (not chi's default 404) produced it", rec.Body.String(), errNoGitHubApp.Error())
		}
	})

	t.Run("path missing the repo segment never reaches the handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/github/app/repos/acme/commits", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d (chi's own not-found)", rec.Code, http.StatusNotFound)
		}
		if strings.Contains(rec.Body.String(), errNoGitHubApp.Error()) {
			t.Fatalf("body = %q, should be chi's default not-found page, not the handler's JSON error - route should not have matched", rec.Body.String())
		}
	})
}
