// Package githubci implements the small GitHub REST surface the CI-box
// feature needs: dispatching and cancelling workflow runs, tracking their
// state, and seeding a dedicated repo with the box workflow file.
package githubci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second}}
}

type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("github: %s (%d)", e.Message, e.Status) }

func (c *Client) do(ctx context.Context, token, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 400 {
		msg := struct {
			Message string `json:"message"`
		}{}
		_ = json.Unmarshal(data, &msg)
		if msg.Message == "" {
			msg.Message = http.StatusText(res.StatusCode)
		}
		return &APIError{Status: res.StatusCode, Message: msg.Message}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return err
		}
	}
	return nil
}

// ValidateToken returns the login the token belongs to.
func (c *Client) ValidateToken(ctx context.Context, token string) (string, error) {
	var me struct {
		Login string `json:"login"`
	}
	if err := c.do(ctx, token, http.MethodGet, "/user", nil, &me); err != nil {
		return "", err
	}
	if me.Login == "" {
		return "", errors.New("github: empty login")
	}
	return me.Login, nil
}

// RepoAccess verifies the token can see the repo and returns its default branch.
func (c *Client) RepoAccess(ctx context.Context, token, owner, repo string) (string, error) {
	var r struct {
		DefaultBranch string `json:"default_branch"`
		Private       bool   `json:"private"`
	}
	if err := c.do(ctx, token, http.MethodGet, "/repos/"+owner+"/"+repo, nil, &r); err != nil {
		return "", err
	}
	if r.DefaultBranch == "" {
		r.DefaultBranch = "main"
	}
	return r.DefaultBranch, nil
}

// CreatePrivateRepo creates a private repo for the token's owner. If it already
// exists (422), the error is returned as-is so callers can fall back to using it.
func (c *Client) CreatePrivateRepo(ctx context.Context, token, name string) error {
	return c.do(ctx, token, http.MethodPost, "/user/repos", map[string]any{
		"name":        name,
		"private":     true,
		"description": "On-demand CI boxes driven by instabox-vm-monitor",
		"auto_init":   true,
	}, nil)
}

// PutFile creates or updates a file in the repo.
func (c *Client) PutFile(ctx context.Context, token, owner, repo, path, contentB64, message, branch string) error {
	// update needs the existing blob sha; fetch is best-effort
	sha := ""
	var cur struct {
		SHA string `json:"sha"`
	}
	if err := c.do(ctx, token, http.MethodGet, fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch), nil, &cur); err == nil {
		sha = cur.SHA
	}
	body := map[string]any{
		"message": message,
		"content": contentB64,
		"branch":  branch,
	}
	if sha != "" {
		body["sha"] = sha
	}
	return c.do(ctx, token, http.MethodPut, fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path), body, nil)
}

// DispatchWorkflow triggers a workflow_dispatch run.
func (c *Client) DispatchWorkflow(ctx context.Context, token, owner, repo, workflowFile, branch string) error {
	err := c.do(ctx, token, http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflowFile),
		map[string]any{"ref": branch}, nil)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNoContent {
			return nil
		}
		return err
	}
	return nil
}

type WorkflowRun struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`     // queued, in_progress, completed, waiting
	Conclusion string `json:"conclusion"` // success, failure, cancelled, ...
	Name       string `json:"name"`
	CreatedAt  string `json:"created_at"`
	HTMLURL    string `json:"html_url"`
}

// ListRecentRuns returns newest-first runs of a workflow file.
func (c *Client) ListRecentRuns(ctx context.Context, token, owner, repo, workflowFile string, perPage int) ([]WorkflowRun, error) {
	var out struct {
		Runs []WorkflowRun `json:"workflow_runs"`
	}
	err := c.do(ctx, token, http.MethodGet,
		fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/runs?per_page=%d", owner, repo, workflowFile, perPage),
		nil, &out)
	return out.Runs, err
}

// FindRunSince locates the run created at or after the given time (dispatch is
// fire-and-forget and returns no run id).
func (c *Client) FindRunSince(ctx context.Context, token, owner, repo, workflowFile string, since time.Time) (*WorkflowRun, error) {
	runs, err := c.ListRecentRuns(ctx, token, owner, repo, workflowFile, 10)
	if err != nil {
		return nil, err
	}
	var best *WorkflowRun
	for i := range runs {
		ct, err := time.Parse(time.RFC3339, runs[i].CreatedAt)
		if err != nil {
			continue
		}
		if ct.Before(since.Add(-5 * time.Second)) {
			continue
		}
		if best == nil || ct.After(mustParseTime(best.CreatedAt)) {
			r := runs[i]
			best = &r
		}
	}
	return best, nil
}

func mustParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// GetRun fetches a single run.
func (c *Client) GetRun(ctx context.Context, token, owner, repo string, runID int64) (*WorkflowRun, error) {
	var out WorkflowRun
	err := c.do(ctx, token, http.MethodGet, fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelRun asks GitHub to cancel the run — the instabox job ends and its box is deleted.
func (c *Client) CancelRun(ctx context.Context, token, owner, repo string, runID int64) error {
	err := c.do(ctx, token, http.MethodPost, fmt.Sprintf("/repos/%s/%s/actions/runs/%d/cancel", owner, repo, runID), map[string]any{}, nil)
	if err != nil {
		var apiErr *APIError
		// 202 Accepted is success; the client treats it as success already, but a 409
		// means the run already finished — not an error for our purposes.
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
			return nil
		}
		return err
	}
	return nil
}

// BoxWorkflowYAML is the workflow that holds a CI box: instabox creates a
// fresh ci machine for the job, the loop keeps it alive (runner traffic is
// real activity), and the job ends when the run is cancelled or the session
// budget runs out — either way instabox deletes the box.
func BoxWorkflowYAML(maxMinutes int) string {
	if maxMinutes <= 0 {
		maxMinutes = 460 // 7h40, under the 8h free-plan session cap
	}
	return fmt.Sprintf(`name: ci-box
on:
  workflow_dispatch:
    inputs:
      note:
        description: Optional note
        required: false
        default: ''

jobs:
  box:
    runs-on: instabox-medium
    timeout-minutes: %d
    steps:
      - name: Hold the box
        run: |
          echo "ci box is up — machine lives until this job ends"
          end=$((SECONDS + %d*60 - 300))
          while [ $SECONDS -lt $end ]; do sleep 60; done
          echo "session budget spent, releasing the box"
`, maxMinutes+10, maxMinutes)
}

// IsNotFound reports a 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

// IsValidationFailed reports a 422 (repo exists, etc).
func IsValidationFailed(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusUnprocessableEntity
}

// OwnerFromRepo splits "owner/repo".
func OwnerFromRepo(full string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(full), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
