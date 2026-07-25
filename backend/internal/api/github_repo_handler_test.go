package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
