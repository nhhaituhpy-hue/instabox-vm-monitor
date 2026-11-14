package instabox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
)

type Action string

const (
	ActionWake    Action = "wake"
	ActionSuspend Action = "suspend"
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionResume  Action = "resume"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, hc *http.Client) (*Client, error) {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		raw = "https://api.instabox.sh"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid instabox api url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, errors.New("instabox api url must use https")
	}
	host := strings.ToLower(u.Hostname())
	if host != "api.instabox.sh" && !strings.HasSuffix(host, ".instabox.sh") {
		return nil, fmt.Errorf("disallowed instabox api host %q", host)
	}
	if hc == nil {
		hc = &http.Client{Timeout: 45 * time.Second}
	}
	clone := *hc
	prevCheck := clone.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if prevCheck != nil {
			if err := prevCheck(req, via); err != nil {
				return err
			}
		}
		return http.ErrUseLastResponse
	}
	return &Client{
		baseURL: strings.TrimRight(u.String(), "/"),
		http:    &clone,
	}, nil
}

type OrgRef struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Tier            string `json:"tier"`
	Kind            string `json:"kind"`
	AlwaysOnAllowed bool   `json:"alwaysOnAllowed,omitempty"`
}

type Me struct {
	UserID                  int64         `json:"userId"`
	Login                   string        `json:"login"`
	Name                    string        `json:"name"`
	AvatarURL               string        `json:"avatarUrl"`
	Email                   string        `json:"email"`
	Admin                   bool          `json:"admin"`
	MachineMetrics          bool          `json:"machineMetrics"`
	OrgID                   string        `json:"orgId"`
	Org                     *OrgRef       `json:"org"`
	Workspaces              []OrgRef      `json:"workspaces,omitempty"`
	AllowedSpecs            []MachineSpec `json:"allowedSpecs,omitempty"`
	AlwaysOnAllowed         bool          `json:"alwaysOnAllowed,omitempty"`
	ManualLifecycle         bool          `json:"manualLifecycle,omitempty"`
	FixedAutoSuspendSeconds int           `json:"fixedAutoSuspendSeconds,omitempty"`
	IdleChoicesSeconds      []int         `json:"idleChoicesSeconds,omitempty"`
}

type Template struct {
	ID          string      `json:"id"`
	Version     string      `json:"version"`
	Description string      `json:"description,omitempty"`
	Arch        string      `json:"arch"`
	DefaultSpec MachineSpec `json:"defaultSpec"`
	OS          string      `json:"os,omitempty"`
	GPU         bool        `json:"gpu,omitempty"`
	Hidden      bool        `json:"hidden,omitempty"`
	Creatable   *bool       `json:"creatable,omitempty"`
	Latest      bool        `json:"latest,omitempty"`
	Default     bool        `json:"default,omitempty"`
	Restricted  bool        `json:"restricted,omitempty"`
}

type TemplateList struct {
	Templates []Template `json:"templates"`
}

type CreateMachineInput struct {
	Name                    string `json:"name,omitempty"`
	TemplateID              string `json:"templateId,omitempty"`
	VCPUs                   int    `json:"vcpus,omitempty"`
	MemMiB                  int    `json:"memMiB,omitempty"`
	DesiredState            string `json:"desiredState,omitempty"`
	AutoSuspendAfterSeconds *int   `json:"autoSuspendAfterSeconds,omitempty"`
	MaxRunMinutes           *int   `json:"maxRunMinutes,omitempty"`
}

type TemplateRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type MachineSpec struct {
	VCPUs             int    `json:"vcpus"`
	MemMiB            int    `json:"memMiB"`
	PersistentDiskGiB int    `json:"persistentDiskGiB"`
	RootDiskGiB       int    `json:"rootDiskGiB,omitempty"`
	GPUs              int    `json:"gpus,omitempty"`
	GPUType           string `json:"gpuType,omitempty"`
}

type Storage struct {
	SizeGiB     int    `json:"sizeGiB"`
	MountPath   string `json:"mountPath"`
	Persistent  bool   `json:"persistent"`
	Synced      bool   `json:"synced"`
	SnapshotSeq int64  `json:"snapshotSeq,omitempty"`
}

type Runtime struct {
	Ready              bool `json:"ready"`
	ProcessesPreserved bool `json:"processesPreserved,omitempty"`
}

type APIErrorBody struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	RequestID         string `json:"requestId"`
	Retryable         bool   `json:"retryable"`
	OperationID       string `json:"operationId,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Hint              string `json:"hint,omitempty"`
	DocsURL           string `json:"docs_url,omitempty"`
}

type APIError struct {
	Status            int
	Code              string
	Message           string
	RequestID         string
	Retryable         bool
	OperationID       string
	RetryAfterSeconds int
	Reason            string
	Hint              string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("instabox api %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("instabox api %d: %s", e.Status, e.Message)
}

type Machine struct {
	ID                      string            `json:"id"`
	Name                    string            `json:"name"`
	DesiredState            string            `json:"desiredState"`
	State                   string            `json:"state"`
	Revision                int64             `json:"revision"`
	OperationID             string            `json:"operationId"`
	Template                TemplateRef       `json:"template"`
	Spec                    MachineSpec       `json:"spec"`
	Storage                 Storage           `json:"storage"`
	Runtime                 Runtime           `json:"runtime"`
	AutoSuspendAfterSeconds *int              `json:"autoSuspendAfterSeconds"`
	AutoSuspendedAt         string            `json:"autoSuspendedAt,omitempty"`
	MaxRunMinutes           int               `json:"maxRunMinutes"`
	RunStartedAt            string            `json:"runStartedAt"`
	CooldownUntil           string            `json:"cooldownUntil"`
	CooldownReason          string            `json:"cooldownReason,omitempty"`
	LastError               *APIErrorBody     `json:"lastError,omitempty"`
	Metadata                map[string]string `json:"metadata"`
	GuestUser               string            `json:"guestUser"`
	OrgID                   string            `json:"orgId"`
	Free                    bool              `json:"free"`
	OOMKills                int64             `json:"oomKills,omitempty"`
	CreatedAt               string            `json:"createdAt"`
	UpdatedAt               string            `json:"updatedAt"`
}

type MachineList struct {
	Machines   []Machine `json:"machines"`
	NextCursor string    `json:"nextCursor"`
}

type MachineFilesystem struct {
	Mount          string  `json:"mount"`
	SizeBytes      int64   `json:"sizeBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	AvailableBytes int64   `json:"availableBytes"`
	UsedPct        float64 `json:"usedPct"`
	InodesUsedPct  float64 `json:"inodesUsedPct"`
	Level          string  `json:"level"`
}

type DiskWarning struct {
	Mount   string  `json:"mount"`
	UsedPct float64 `json:"usedPct"`
	Level   string  `json:"level"`
	Message string  `json:"message"`
	NoSpace bool    `json:"noSpace,omitempty"`
}

type MachineDisk struct {
	MeasuredAt  string              `json:"measuredAt"`
	Filesystems []MachineFilesystem `json:"filesystems"`
	Warning     *DiskWarning        `json:"warning,omitempty"`
}

type MachineMetric struct {
	Timestamp            string  `json:"timestamp"`
	CPUCount             int     `json:"cpuCount"`
	CPUUsedPct           float64 `json:"cpuUsedPct"`
	MemUsed              int64   `json:"memUsed"`
	MemTotal             int64   `json:"memTotal"`
	DiskTotal            int64   `json:"diskTotal"`
	DiskUsed             int64   `json:"diskUsed,omitempty"`
	DiskReadBytesPerSec  float64 `json:"diskReadBytesPerSec,omitempty"`
	DiskWriteBytesPerSec float64 `json:"diskWriteBytesPerSec,omitempty"`
	NetInBytesPerSec     float64 `json:"netInBytesPerSec,omitempty"`
	NetOutBytesPerSec    float64 `json:"netOutBytesPerSec,omitempty"`
}

type MachineMetrics struct {
	Disk    *MachineDisk    `json:"disk,omitempty"`
	Metrics []MachineMetric `json:"metrics"`
}

type MachineUsage struct {
	From           string  `json:"from"`
	To             string  `json:"to"`
	RunningSeconds int64   `json:"runningSeconds"`
	VCPUSeconds    int64   `json:"vcpuSeconds"`
	MemGiBSeconds  float64 `json:"memGiBSeconds"`
	DiskGiBSeconds int64   `json:"diskGiBSeconds"`
	EgressBytes    int64   `json:"egressBytes"`
}

type Operation struct {
	ID        string        `json:"id"`
	MachineID string        `json:"machineId"`
	Phase     string        `json:"phase,omitempty"`
	State     string        `json:"state"`
	Trigger   string        `json:"trigger,omitempty"`
	Error     *APIErrorBody `json:"error,omitempty"`
	Machine   *Machine      `json:"machine,omitempty"`
	CreatedAt string        `json:"createdAt"`
}

type ActionResult struct {
	Operation *Operation `json:"operation,omitempty"`
	Machine   *Machine   `json:"machine,omitempty"`
}

func (c *Client) GetMe(ctx context.Context, apiKey string) (*Me, error) {
	var out Me
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/me", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetUsageRaw(ctx context.Context, apiKey string) (json.RawMessage, error) {
	var raw json.RawMessage
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/usage", apiKey, nil, false, &raw)
	return raw, err
}

func (c *Client) ListAllMachines(ctx context.Context, apiKey string) ([]Machine, time.Time, error) {
	all := make([]Machine, 0)
	cursor := ""
	var serverTime time.Time
	for range 25 {
		path := "/v1/machines?limit=100"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		var page MachineList
		srvTime, err := c.doJSON(ctx, http.MethodGet, path, apiKey, nil, false, &page)
		if err != nil {
			return nil, time.Time{}, err
		}
		if !srvTime.IsZero() {
			serverTime = srvTime
		}
		all = append(all, page.Machines...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, serverTime, nil
}

func (c *Client) GetMachine(ctx context.Context, apiKey, machineID string) (*Machine, error) {
	var out Machine
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+url.PathEscape(machineID), apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMachineMetrics(ctx context.Context, apiKey, machineID string, since time.Time) (*MachineMetrics, error) {
	path := "/v1/machines/" + url.PathEscape(machineID) + "/metrics"
	if !since.IsZero() {
		// instabox.sh strictly rejects `since` older than 24h (and RFC3339 truncates subseconds).
		// Clamp conservatively to 23h45m ago so clock skew / rounding never triggers 400 bad_request.
		floor := time.Now().UTC().Add(-23*time.Hour - 45*time.Minute)
		if since.UTC().Before(floor) {
			since = floor
		}
		path += "?since=" + url.QueryEscape(since.UTC().Format(time.RFC3339))
	}
	var out MachineMetrics
	_, err := c.doJSON(ctx, http.MethodGet, path, apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMachineDisk(ctx context.Context, apiKey, machineID string) (*MachineDisk, error) {
	var out MachineDisk
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+url.PathEscape(machineID)+"/disk", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMachineUsage(ctx context.Context, apiKey, machineID string) (*MachineUsage, error) {
	var out MachineUsage
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+url.PathEscape(machineID)+"/usage", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListTemplates(ctx context.Context, apiKey string) ([]Template, error) {
	var out TemplateList
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/templates", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return out.Templates, nil
}

func (c *Client) CreateMachine(ctx context.Context, apiKey string, in CreateMachineInput) (*ActionResult, error) {
	var op Operation
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/machines", apiKey, in, true, &op)
	if err != nil {
		return nil, err
	}
	return &ActionResult{Operation: &op}, nil
}

func (c *Client) GetOperation(ctx context.Context, apiKey, opID string, wait string) (*Operation, error) {
	path := "/v1/operations/" + url.PathEscape(opID)
	if wait != "" {
		path += "?wait=" + url.QueryEscape(wait)
	}
	var out Operation
	_, err := c.doJSON(ctx, http.MethodGet, path, apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PerformAction(ctx context.Context, apiKey, machineID string, action Action) (*ActionResult, error) {
	switch action {
	case ActionWake:
		var m Machine
		_, err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+url.PathEscape(machineID)+"/wake", apiKey, map[string]any{}, false, &m)
		if err != nil {
			return nil, err
		}
		return &ActionResult{Machine: &m}, nil
	case ActionStart, ActionStop, ActionSuspend, ActionResume:
		var op Operation
		_, err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+url.PathEscape(machineID)+"/"+string(action), apiKey, map[string]any{}, true, &op)
		if err != nil {
			return nil, err
		}
		return &ActionResult{Operation: &op}, nil
	default:
		return nil, fmt.Errorf("unsupported action %q", action)
	}
}

func (c *Client) doJSON(ctx context.Context, method, path, apiKey string, body any, needIdem bool, out any) (time.Time, error) {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return time.Time{}, err
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return time.Time{}, err
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "instabox-vm-monitor/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if needIdem {
		req.Header.Set("Idempotency-Key", crypto.NewUUID())
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	var serverTime time.Time
	if dateHdr := resp.Header.Get("Date"); dateHdr != "" {
		if parsed, parseErr := http.ParseTime(dateHdr); parseErr == nil {
			serverTime = parsed.UTC()
		}
	}

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return serverTime, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			return serverTime, &APIError{
				Status:  http.StatusBadGateway,
				Code:    "unexpected_redirect",
				Message: fmt.Sprintf("unexpected redirect (%d) from upstream API", resp.StatusCode),
			}
		}
		var env struct {
			Error APIErrorBody `json:"error"`
		}
		_ = json.Unmarshal(rawBody, &env)
		apiErr := &APIError{
			Status:            resp.StatusCode,
			Code:              env.Error.Code,
			Message:           env.Error.Message,
			RequestID:         env.Error.RequestID,
			Retryable:         env.Error.Retryable,
			OperationID:       env.Error.OperationID,
			RetryAfterSeconds: env.Error.RetryAfterSeconds,
			Reason:            env.Error.Reason,
			Hint:              env.Error.Hint,
		}
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
		return serverTime, apiErr
	}

	if out != nil && len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, out); err != nil {
			return serverTime, fmt.Errorf("decode response: %w", err)
		}
	}
	return serverTime, nil
}
