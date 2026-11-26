package monitor

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/githubci"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

// CI-box lifecycle, driven through GitHub Actions because instabox.sh reserves
// free-plan ci machines for Actions jobs:
//
//   idle ──start──▶ dispatching ──▶ starting ──▶ running ──stop──▶ stopping ──▶ idle
//
// - dispatching: workflow_dispatch fired, waiting for GitHub to create the run
// - starting:    run exists, waiting for instabox to hand the job its CI box
// - running:     box is up; machineId known via the org's jobs feed
// - stopping:    cancel requested, waiting for instabox to delete the box
//
// The poller drives state transitions; nothing blocks a poll cycle.

const (
	ciBoxMaxMinutes        = 460 // 7h40 — under the 8h free-plan session cap
	ciBoxDispatchGraceSec  = 120 // time for GitHub to register a dispatch
	ciBoxJobMatchWindowSec = 600 // a jobs-feed entry matches if created within 10m of the dispatch
)

var (
	ErrCIBoxNotConfigured = errors.New("ci box is not configured for this account")
	ErrCIBoxBusy          = errors.New("ci box is already starting or running")
)

// SetupCIBox validates the token and repo, ensures the workflow file exists,
// and stores the configuration. When repo is empty, a dedicated private repo
// named ci-box is created for the token's owner.
func (p *Poller) SetupCIBox(ctx context.Context, accountID, repo, token string) (*store.CIBox, error) {
	if token == "" {
		// reconfigure with the stored token
		var err error
		token, err = p.store.GetCIBoxToken(ctx, accountID)
		if err != nil || token == "" {
			return nil, errors.New("a GitHub token is required")
		}
	}
	gh := githubci.NewClient()
	login, err := gh.ValidateToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("github token check failed: %w", err)
	}

	owner, repoName := "", ""
	if repo != "" {
		var ok bool
		owner, repoName, ok = githubci.OwnerFromRepo(repo)
		if !ok {
			return nil, errors.New("repo must be in owner/name form")
		}
	} else {
		owner = login
		repoName = "ci-box"
		if err := gh.CreatePrivateRepo(ctx, token, repoName); err != nil && !githubci.IsValidationFailed(err) {
			return nil, fmt.Errorf("creating the ci-box repo failed: %w", err)
		}
		repo = owner + "/" + repoName
	}

	branch, err := gh.RepoAccess(ctx, token, owner, repoName)
	if err != nil {
		return nil, fmt.Errorf("token cannot see %s (needs actions+contents access): %w", repo, err)
	}

	yaml := githubci.BoxWorkflowYAML(ciBoxMaxMinutes)
	if err := gh.PutFile(ctx, token, owner, repoName, ".github/workflows/ci-box.yml",
		base64.StdEncoding.EncodeToString([]byte(yaml)), "ci: box workflow", branch); err != nil {
		return nil, fmt.Errorf("writing the workflow file failed: %w", err)
	}

	return p.store.UpsertCIBox(ctx, accountID, repo, "ci-box.yml", branch, token, login)
}

// StartCIBox dispatches the workflow — instabox will create a fresh ci machine
// for the job within a few seconds.
func (p *Poller) StartCIBox(ctx context.Context, accountID string) (*store.CIBox, error) {
	box, err := p.store.GetCIBox(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if box == nil {
		return nil, ErrCIBoxNotConfigured
	}
	if box.State == "dispatching" || box.State == "starting" || box.State == "running" {
		return nil, ErrCIBoxBusy
	}
	token, err := p.store.GetCIBoxToken(ctx, accountID)
	if err != nil || token == "" {
		return nil, errors.New("github token is missing — reconfigure")
	}
	owner, repoName, ok := githubci.OwnerFromRepo(box.Repo)
	if !ok {
		return nil, errors.New("stored repo is invalid — reconfigure")
	}
	gh := githubci.NewClient()
	if err := gh.DispatchWorkflow(ctx, token, owner, repoName, box.WorkflowFile, box.Branch); err != nil {
		msg := "workflow dispatch failed: " + err.Error()
		_ = p.store.UpdateCIBoxState(ctx, box.ID, "error", box.RunID, box.RunURL, box.MachineID, msg)
		return nil, errors.New(msg)
	}
	// remember when the dispatch happened so the run search and the jobs-feed
	// match only look at things created after it
	dispatchedAt := time.Now().Unix()
	_ = dispatchedAt // the run search re-reads createdAt; see pollCIBoxes
	if err := p.store.UpdateCIBoxState(ctx, box.ID, "dispatching", 0, "", "", ""); err != nil {
		return nil, err
	}
	return p.store.GetCIBox(ctx, accountID)
}

// StopCIBox cancels the workflow run — the job ends and instabox deletes the box.
func (p *Poller) StopCIBox(ctx context.Context, accountID string) (*store.CIBox, error) {
	box, err := p.store.GetCIBox(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if box == nil {
		return nil, ErrCIBoxNotConfigured
	}
	if box.State == "idle" || box.State == "error" {
		return box, nil // nothing to stop
	}
	token, err := p.store.GetCIBoxToken(ctx, accountID)
	if err != nil || token == "" {
		return nil, errors.New("github token is missing — reconfigure")
	}
	if box.RunID != 0 {
		owner, repoName, _ := githubci.OwnerFromRepo(box.Repo)
		gh := githubci.NewClient()
		if err := gh.CancelRun(ctx, token, owner, repoName, box.RunID); err != nil {
			msg := "cancelling the run failed: " + err.Error()
			_ = p.store.UpdateCIBoxState(ctx, box.ID, "error", box.RunID, box.RunURL, box.MachineID, msg)
			return nil, errors.New(msg)
		}
	}
	if err := p.store.UpdateCIBoxState(ctx, box.ID, "stopping", box.RunID, box.RunURL, box.MachineID, ""); err != nil {
		return nil, err
	}
	return p.store.GetCIBox(ctx, accountID)
}

// DeleteCIBoxConfig removes the configuration (after stopping any active box).
func (p *Poller) DeleteCIBoxConfig(ctx context.Context, accountID string) error {
	box, err := p.store.GetCIBox(ctx, accountID)
	if err != nil {
		return err
	}
	if box == nil {
		return nil
	}
	if box.State != "idle" && box.State != "error" {
		if _, err := p.StopCIBox(ctx, accountID); err != nil {
			return err
		}
	}
	return p.store.DeleteCIBox(ctx, accountID)
}

// pollCIBoxes advances every configured ci box one step. Called inline in the
// account sync loop, so transitions are sequential per machine — the same
// single-controller discipline as schedule maintenance.
func (p *Poller) pollCIBoxes(ctx context.Context) {
	boxes, err := p.store.ListCIBoxes(ctx)
	if err != nil || len(boxes) == 0 {
		return
	}
	for i := range boxes {
		p.pollCIBox(ctx, &boxes[i])
	}
}

func (p *Poller) pollCIBox(ctx context.Context, box *store.CIBox) {
	token, err := p.store.GetCIBoxToken(ctx, box.AccountID)
	if err != nil || token == "" {
		return // cannot progress without a token; setup errors surface at setup time
	}
	owner, repoName, ok := githubci.OwnerFromRepo(box.Repo)
	if !ok {
		return
	}
	gh := githubci.NewClient()

	switch box.State {
	case "dispatching":
		// find the run the dispatch created; give GitHub a grace period first
		since := time.Unix(box.UpdatedAt, 0)
		if time.Since(since) < 15*time.Second {
			return
		}
		run, err := gh.FindRunSince(ctx, token, owner, repoName, box.WorkflowFile, since)
		if err != nil || run == nil {
			if time.Since(since) > ciBoxDispatchGraceSec*time.Second {
				_ = p.store.UpdateCIBoxState(ctx, box.ID, "error", 0, "", "", "no workflow run appeared after the dispatch — is the workflow enabled?")
			}
			return
		}
		_ = p.store.UpdateCIBoxState(ctx, box.ID, "starting", run.ID, run.HTMLURL, "", "")

	case "starting", "running":
		// the run may have ended underneath us (cancellation from GitHub UI,
		// timeout, failure) — reconcile, then look for the machine id
		run, err := gh.GetRun(ctx, token, owner, repoName, box.RunID)
		if err == nil && run != nil && run.Status == "completed" {
			concl := run.Conclusion
			if box.MachineID != "" || box.State == "running" {
				// the box lived its life and is gone now — clean end
				_ = p.store.UpdateCIBoxState(ctx, box.ID, "idle", 0, "", "", "")
			} else {
				_ = p.store.UpdateCIBoxState(ctx, box.ID, "error", 0, "", "", "the job ended before a box was assigned ("+concl+")")
			}
			return
		}
		if box.MachineID != "" {
			if box.State != "running" {
				_ = p.store.UpdateCIBoxState(ctx, box.ID, "running", box.RunID, box.RunURL, box.MachineID, "")
			}
			return
		}
		// ask instabox which machine the job got: match by the org's jobs feed
		acc, accErr := p.store.GetAccount(ctx, box.AccountID)
		if accErr != nil || acc.OrgID == "" {
			return
		}
		apiKey, keyErr := p.store.GetAccountAPIKey(ctx, box.AccountID)
		if keyErr != nil {
			return
		}
		jobs, jobsErr := p.client.ListGitHubJobs(ctx, apiKey, acc.OrgID, 20)
		if jobsErr != nil {
			return
		}
		since := time.Unix(box.UpdatedAt, 0).Add(-ciBoxJobMatchWindowSec * time.Second)
		for _, job := range jobs {
			if job.MachineID == "" {
				continue
			}
			if job.State == "completed" || job.State == "failed" || job.State == "refused" {
				continue
			}
			ct, _ := time.Parse(time.RFC3339, job.CreatedAt)
			if ct.Before(since) && since.Unix() > 0 {
				continue
			}
			_ = p.store.UpdateCIBoxState(ctx, box.ID, "running", box.RunID, box.RunURL, job.MachineID, "")
			return
		}

	case "stopping":
		run, err := gh.GetRun(ctx, token, owner, repoName, box.RunID)
		if err != nil {
			return
		}
		if run.Status == "completed" {
			_ = p.store.UpdateCIBoxState(ctx, box.ID, "idle", 0, "", "", "")
		}
	}
}
