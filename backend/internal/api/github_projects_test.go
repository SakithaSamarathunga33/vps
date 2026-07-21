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
	if summary.PullRequests == nil || summary.Issues == nil {
		t.Errorf("PullRequests/Issues should be empty slices, not nil, so JSON encodes [] not null")
	}
}

// TestBuildProjectSummaryPartialFailure simulates what githubAppProjects now
// passes when a single per-repo GitHub call fails (e.g. ListOpenPullRequests
// errors because the caller lacks PR access): that one field degrades to its
// nil/empty value while the rest of the summary keeps whatever data did
// succeed. This proves a single failing sub-call no longer drops the whole
// repo from the aggregated result.
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
	if summary.PullRequests == nil {
		t.Errorf("PullRequests should be an empty slice, not nil, so JSON encodes [] not null")
	}
	if len(summary.PullRequests) != 0 {
		t.Errorf("PullRequests = %+v, want empty", summary.PullRequests)
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
