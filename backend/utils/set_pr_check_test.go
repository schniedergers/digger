package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/assert"
)

func newCheckRunRecorder(t *testing.T) (*github2.GithubService, *[]string) {
	created := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/o/r/check-runs" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		created = append(created, body.Name)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": %d}`, len(created))
	}))
	t.Cleanup(srv.Close)
	client := github.NewClient(nil)
	client.BaseURL, _ = client.BaseURL.Parse(srv.URL + "/")
	return &github2.GithubService{Client: client, RepoName: "r", Owner: "o"}, &created
}

func TestSetPRCheckForJobsApplyStatusCheck(t *testing.T) {
	applyJobs := []scheduler.Job{{ProjectName: "p1", Commands: []string{"digger apply"}}}
	planJobs := []scheduler.Job{{ProjectName: "p1", Commands: []string{"digger plan"}}}

	cases := []struct {
		name         string
		jobs         []scheduler.Job
		disable      bool
		wantChecks   []string
		wantBatchSet bool
	}{
		{"apply enabled", applyJobs, false, []string{"p1/apply", "digger/apply"}, true},
		{"apply disabled", applyJobs, true, []string{"p1/apply"}, false},
		{"plan enabled", planJobs, false, []string{"p1/plan", "digger/plan", "digger/apply"}, true},
		{"plan disabled", planJobs, true, []string{"p1/plan", "digger/plan"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, created := newCheckRunRecorder(t)
			batch, jobChecks, err := SetPRCheckForJobs(svc, 1, tc.jobs, "sha", "r", "o", tc.disable)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantChecks, *created)
			assert.Equal(t, tc.wantBatchSet, batch.Id != "")
			assert.Contains(t, jobChecks, "p1")
		})
	}
}
