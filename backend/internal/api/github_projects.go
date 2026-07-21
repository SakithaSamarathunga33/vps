package api

import (
	"net/http"
	"strconv"
	"strings"

	"pulsenode/backend/internal/github"
)

// ProjectSummary is one repo's aggregated GitHub data: metadata plus open
// PRs/issues and the latest commit/workflow run. Returned by githubAppProjects
// so a caller (Corevia) can build a repository dashboard from a single HTTP
// round trip instead of one GitHub API call per data point.
type ProjectSummary struct {
	ExternalID     string               `json:"external_id"`
	FullName       string               `json:"full_name"`
	Description    string               `json:"description"`
	Language       string               `json:"language"`
	DefaultBranch  string               `json:"default_branch"`
	LatestCommit   *github.Commit       `json:"latest_commit"`
	LatestWorkflow *github.WorkflowRun  `json:"latest_workflow"`
	PullRequests   []github.PullRequest `json:"pull_requests"`
	Issues         []github.Issue       `json:"issues"`
}

// buildProjectSummary assembles a ProjectSummary from already-fetched data.
// Kept separate from githubAppProjects (which does the DB/GitHub I/O) so it's
// testable without a fake GitHub server.
func buildProjectSummary(repo github.Repo, prs []github.PullRequest, issues []github.Issue, commit *github.Commit, workflow *github.WorkflowRun) ProjectSummary {
	if prs == nil {
		prs = []github.PullRequest{}
	}
	if issues == nil {
		issues = []github.Issue{}
	}
	return ProjectSummary{
		ExternalID:     strconv.FormatInt(repo.ID, 10),
		FullName:       repo.FullName,
		Description:    repo.Description,
		Language:       repo.Language,
		DefaultBranch:  repo.DefaultBranch,
		LatestCommit:   commit,
		LatestWorkflow: workflow,
		PullRequests:   prs,
		Issues:         issues,
	}
}

// githubAppProjects aggregates a full GitHub summary (metadata, open PRs,
// open issues, latest commit, latest workflow run) for every repo across all
// stored GitHub App installations. An installation that errors (minting its
// token or listing its repos) is skipped entirely, matching githubAppRepos.
// Within a repo, each of the four per-repo GitHub calls degrades
// independently: a failure (e.g. Issues or Actions disabled on that repo)
// yields an empty/nil value for that field instead of dropping the whole
// repo from the response.
func (s *Server) githubAppProjects(w http.ResponseWriter, r *http.Request) {
	projects := []ProjectSummary{}

	appID, _ := s.db.GetSetting("github_app_id")
	pkPEM, _ := s.db.GetSetting("github_app_private_key")
	if appID == "" || pkPEM == "" {
		writeJSON(w, http.StatusOK, projects)
		return
	}
	appClient, err := github.NewAppClient(appID, pkPEM)
	if err != nil {
		writeError(w, err)
		return
	}
	insts, err := s.db.ListAppInstallations()
	if err != nil {
		writeError(w, err)
		return
	}

	for _, inst := range insts {
		tok, err := appClient.GetInstallationToken(inst.InstallationID)
		if err != nil {
			continue
		}
		repos, err := github.InstallationRepos(tok.Token)
		if err != nil {
			continue
		}
		client := github.NewClient(tok.Token)
		for _, repo := range repos {
			parts := strings.SplitN(repo.FullName, "/", 2)
			if len(parts) != 2 {
				continue
			}
			owner, name := parts[0], parts[1]

			prs, err := client.ListOpenPullRequests(owner, name)
			if err != nil {
				prs = nil
			}
			issues, err := client.ListOpenIssues(owner, name)
			if err != nil {
				issues = nil
			}
			commit, err := client.GetLatestCommit(owner, name)
			if err != nil {
				commit = nil
			}
			workflow, err := client.GetLatestWorkflowRun(owner, name)
			if err != nil {
				workflow = nil
			}

			projects = append(projects, buildProjectSummary(repo, prs, issues, commit, workflow))
		}
	}
	writeJSON(w, http.StatusOK, projects)
}
