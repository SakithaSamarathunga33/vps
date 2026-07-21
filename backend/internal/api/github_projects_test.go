package api

import (
	"testing"

	"pulsenode/backend/internal/github"
)

func TestBuildProjectSummary(t *testing.T) {
	repo := github.Repo{ID: 42, FullName: "acme/widgets", Description: "A widget factory", Language: "Go", DefaultBranch: "main"}
	prs := []github.PullRequest{{ExternalID: "1", Number: 7, Title: "Add feature", State: "open", Author: "octocat", URL: "https://github.com/acme/widgets/pull/7"}}
	issues := []github.Issue{{ExternalID: "2", Number: 3, Title: "Bug", State: "open", URL: "https://github.com/acme/widgets/issues/3"}}
	commit := &github.Commit{SHA: "abc123def456", Message: "Fix bug"}
	workflow := &github.WorkflowRun{Status: "completed", Conclusion: "success"}

	summary := buildProjectSummary(repo, prs, issues, commit, workflow)

	if summary.ExternalID != "42" {
		t.Errorf("ExternalID = %q, want 42", summary.ExternalID)
	}
	if summary.FullName != "acme/widgets" || summary.Description != "A widget factory" || summary.Language != "Go" || summary.DefaultBranch != "main" {
		t.Errorf("summary metadata mismatch: %+v", summary)
	}
	if len(summary.PullRequests) != 1 || summary.PullRequests[0] != prs[0] {
		t.Errorf("PullRequests mismatch: %+v", summary.PullRequests)
	}
	if len(summary.Issues) != 1 || summary.Issues[0] != issues[0] {
		t.Errorf("Issues mismatch: %+v", summary.Issues)
	}
	if summary.LatestCommit == nil || *summary.LatestCommit != *commit {
		t.Errorf("LatestCommit mismatch: %+v", summary.LatestCommit)
	}
	if summary.LatestWorkflow == nil || *summary.LatestWorkflow != *workflow {
		t.Errorf("LatestWorkflow mismatch: %+v", summary.LatestWorkflow)
	}
}

func TestBuildProjectSummaryNilCommitAndWorkflow(t *testing.T) {
	repo := github.Repo{ID: 1, FullName: "acme/empty"}
	summary := buildProjectSummary(repo, nil, nil, nil, nil)

	if summary.LatestCommit != nil {
		t.Errorf("LatestCommit = %+v, want nil", summary.LatestCommit)
	}
	if summary.LatestWorkflow != nil {
		t.Errorf("LatestWorkflow = %+v, want nil", summary.LatestWorkflow)
	}
}

// TestBuildProjectSummaryNilPullRequestsAndIssues covers the contract that
// distinguishes "fetch failed" from "fetch succeeded with zero results": a
// nil prs/issues input (what githubAppProjects passes when
// ListOpenPullRequests/ListOpenIssues errors for a repo) must pass through as
// nil in the result, so it serializes as JSON null rather than []. A JSON []
// is reserved for a successful fetch that genuinely found zero open
// PRs/issues (the GitHub client methods always return a non-nil, possibly
// empty, slice on success). Corevia's sync logic treats null as
// "unknown/unavailable, leave existing data alone" and [] as "confirmed
// empty, safe to reconcile" - coalescing nil to [] here would silently turn
// a transient fetch failure into a false "this repo has zero open PRs/issues"
// signal and wipe previously-synced rows.
func TestBuildProjectSummaryNilPullRequestsAndIssues(t *testing.T) {
	repo := github.Repo{ID: 1, FullName: "acme/empty"}
	summary := buildProjectSummary(repo, nil, nil, nil, nil)

	if summary.PullRequests != nil {
		t.Errorf("PullRequests = %+v, want nil (JSON null), since prs input was nil", summary.PullRequests)
	}
	if summary.Issues != nil {
		t.Errorf("Issues = %+v, want nil (JSON null), since issues input was nil", summary.Issues)
	}
}

// TestBuildProjectSummaryPartialFailure simulates what githubAppProjects
// passes when a single per-repo GitHub call fails (e.g. ListOpenPullRequests
// errors because the caller lacks PR access): that one field must degrade to
// nil (unknown/unavailable) while the rest of the summary keeps whatever data
// did succeed. This proves a single failing sub-call neither drops the whole
// repo from the aggregated result nor gets mistaken for a confirmed-empty
// result.
func TestBuildProjectSummaryPartialFailure(t *testing.T) {
	repo := github.Repo{ID: 99, FullName: "acme/partial", Description: "Partial data repo", Language: "Go", DefaultBranch: "main"}
	issues := []github.Issue{{ExternalID: "5", Number: 9, Title: "Bug", State: "open", URL: "https://github.com/acme/partial/issues/9"}}
	commit := &github.Commit{SHA: "deadbeef", Message: "Initial commit"}
	workflow := &github.WorkflowRun{Status: "completed", Conclusion: "success"}

	// prs is nil, as if ListOpenPullRequests had errored for this repo.
	summary := buildProjectSummary(repo, nil, issues, commit, workflow)

	if summary.FullName != "acme/partial" {
		t.Errorf("FullName = %q, want acme/partial", summary.FullName)
	}
	if summary.PullRequests != nil {
		t.Errorf("PullRequests = %+v, want nil (JSON null) since the fetch failed, not an empty slice", summary.PullRequests)
	}
	if len(summary.Issues) != 1 || summary.Issues[0] != issues[0] {
		t.Errorf("Issues mismatch: %+v, want surviving data %+v", summary.Issues, issues)
	}
	if summary.LatestCommit == nil || *summary.LatestCommit != *commit {
		t.Errorf("LatestCommit mismatch: %+v, want surviving data %+v", summary.LatestCommit, commit)
	}
	if summary.LatestWorkflow == nil || *summary.LatestWorkflow != *workflow {
		t.Errorf("LatestWorkflow mismatch: %+v, want surviving data %+v", summary.LatestWorkflow, workflow)
	}
}
