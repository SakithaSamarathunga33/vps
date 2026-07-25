package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"pulsenode/backend/internal/github"
)

var (
	errNoGitHubApp      = errors.New("github app is not configured")
	errRepoNotInstalled = errors.New("repository is not accessible through any installation")
)

// repoClient mints an installation-token client that can see owner/repo and
// returns the repo's metadata (for its default branch). It walks every stored
// installation because one PulseNode may hold several.
func (s *Server) repoClient(owner, repo string) (*github.Client, github.Repo, error) {
	appID, _ := s.db.GetSetting("github_app_id")
	pkPEM, _ := s.db.GetSetting("github_app_private_key")
	if appID == "" || pkPEM == "" {
		return nil, github.Repo{}, errNoGitHubApp
	}
	appClient, err := github.NewAppClient(appID, pkPEM)
	if err != nil {
		return nil, github.Repo{}, err
	}
	insts, err := s.db.ListAppInstallations()
	if err != nil {
		return nil, github.Repo{}, err
	}
	want := strings.ToLower(owner + "/" + repo)
	for _, inst := range insts {
		tok, err := appClient.GetInstallationToken(inst.InstallationID)
		if err != nil {
			continue
		}
		repos, err := github.InstallationRepos(tok.Token)
		if err != nil {
			continue
		}
		for _, r := range repos {
			if strings.ToLower(r.FullName) == want {
				return github.NewClient(tok.Token), r, nil
			}
		}
	}
	return nil, github.Repo{}, errRepoNotInstalled
}

// writeGitHubError maps errors onto meaningful statuses so Corevia can tell
// a stale-sha conflict (409) or missing write permission (403) apart from
// PulseNode being broken.
func writeGitHubError(w http.ResponseWriter, err error) {
	var apiErr *github.APIStatusError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusConflict:
			writeJSON(w, http.StatusConflict, map[string]string{"error": "file changed on GitHub since it was loaded"})
		case http.StatusForbidden:
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "github app lacks contents write permission"})
		case http.StatusNotFound:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found on github"})
		default:
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": apiErr.Error()})
		}
		return
	}
	if errors.Is(err, errRepoNotInstalled) || errors.Is(err, errNoGitHubApp) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeError(w, err)
}

func (s *Server) githubRepoCommits(w http.ResponseWriter, r *http.Request) {
	owner, repo := chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
	client, _, err := s.repoClient(owner, repo)
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	commits, err := client.ListRecentCommits(owner, repo, limit)
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commits)
}

func (s *Server) githubRepoTree(w http.ResponseWriter, r *http.Request) {
	owner, repo := chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
	client, _, err := s.repoClient(owner, repo)
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	entries, err := client.GetTree(owner, repo, r.URL.Query().Get("ref"))
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) githubRepoFile(w http.ResponseWriter, r *http.Request) {
	owner, repo := chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path query parameter is required"})
		return
	}
	client, _, err := s.repoClient(owner, repo)
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	file, err := client.GetFile(owner, repo, path, r.URL.Query().Get("ref"))
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (s *Server) githubRepoUpdateFile(w http.ResponseWriter, r *http.Request) {
	owner, repo := chi.URLParam(r, "owner"), chi.URLParam(r, "repo")
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		SHA     string `json:"sha"`
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Path == "" || body.SHA == "" || body.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path, sha, and message are required"})
		return
	}
	client, repoMeta, err := s.repoClient(owner, repo)
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	res, err := client.UpdateFile(owner, repo, body.Path, repoMeta.DefaultBranch, body.Message, body.Content, body.SHA)
	if err != nil {
		writeGitHubError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
