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
