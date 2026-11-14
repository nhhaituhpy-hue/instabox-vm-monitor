package instabox

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/ws"
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
	baseURL  string
	http     *http.Client
	longHTTP *http.Client
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
	longClone := clone
	if longClone.Timeout < 630*time.Second {
		longClone.Timeout = 630 * time.Second
	}
	return &Client{
		baseURL:  strings.TrimRight(u.String(), "/"),
		http:     &clone,
		longHTTP: &longClone,
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
	Name                    string            `json:"name,omitempty"`
	TemplateID              string            `json:"templateId,omitempty"`
	SnapshotID              string            `json:"snapshotId,omitempty"`
	VCPUs                   int               `json:"vcpus,omitempty"`
	MemMiB                  int               `json:"memMiB,omitempty"`
	DesiredState            string            `json:"desiredState,omitempty"`
	AutoSuspendAfterSeconds *int              `json:"autoSuspendAfterSeconds,omitempty"`
	MaxRunMinutes           *int              `json:"maxRunMinutes,omitempty"`
	Env                     map[string]string `json:"env,omitempty"`
}

type UpdateMachineInput struct {
	Name                    *string           `json:"name,omitempty"`
	AutoSuspendAfterSeconds *int              `json:"autoSuspendAfterSeconds,omitempty"`
	MaxRunMinutes           *int              `json:"maxRunMinutes,omitempty"`
	Metadata                map[string]string `json:"metadata,omitempty"`
	Apply                   string            `json:"apply,omitempty"`
}

type ExecInput struct {
	Cmd            string            `json:"cmd,omitempty"`
	Argv           []string          `json:"argv,omitempty"`
	Cwd            string            `json:"cwd,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	Stdin          string            `json:"stdin,omitempty"`
	TimeoutSeconds int               `json:"timeoutSeconds,omitempty"`
}

type ExecResult struct {
	Stdout      string       `json:"stdout"`
	Stderr      string       `json:"stderr"`
	ExitCode    int          `json:"exitCode"`
	TimedOut    bool         `json:"timedOut"`
	Truncated   bool         `json:"truncated"`
	InvalidUTF8 bool         `json:"invalidUtf8,omitempty"`
	OOMKilled   bool         `json:"oomKilled,omitempty"`
	Signal      string       `json:"signal,omitempty"`
	DiskWarning *DiskWarning `json:"diskWarning,omitempty"`
}

type FileEntry struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"modTime"`
}

type MachineFiles struct {
	Path      string      `json:"path"`
	Type      string      `json:"type"`
	Size      int64       `json:"size"`
	Mode      string      `json:"mode"`
	ModTime   string      `json:"modTime"`
	Content   string      `json:"content,omitempty"`
	Binary    bool        `json:"binary,omitempty"`
	Truncated bool        `json:"truncated,omitempty"`
	Entries   []FileEntry `json:"entries,omitempty"`
}

type FileWritten struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Mode    string `json:"mode"`
	Created bool   `json:"created"`
}

type PublishPortInput struct {
	Port        int    `json:"port"`
	Name        string `json:"name,omitempty"`
	Access      string `json:"access,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	RotateToken bool   `json:"rotateToken,omitempty"`
}

type PublicPort struct {
	ID        string `json:"id"`
	MachineID string `json:"machineId"`
	Port      int    `json:"port"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Address   string `json:"address,omitempty"`
	Access    string `json:"access"`
	Protocol  string `json:"protocol,omitempty"`
	Token     string `json:"token,omitempty"`
	TokenURL  string `json:"tokenUrl,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type PublicPortList struct {
	Enabled bool         `json:"enabled"`
	Ports   []PublicPort `json:"ports"`
}

type ProcessList struct {
	MachineID  string            `json:"machineId"`
	State      string            `json:"state"`
	Live       bool              `json:"live"`
	CapturedAt string            `json:"capturedAt"`
	Trigger    string            `json:"trigger"`
	Truncated  bool              `json:"truncated"`
	Processes  []json.RawMessage `json:"processes"`
}

type MachineLogs struct {
	MachineID          string      `json:"machineId"`
	State              string      `json:"state"`
	Console            string      `json:"console"`
	ConsoleTruncated   bool        `json:"consoleTruncated,omitempty"`
	ConsoleReset       bool        `json:"consoleReset,omitempty"`
	ConsoleUnavailable string      `json:"consoleUnavailable,omitempty"`
	Cursor             int64       `json:"cursor"`
	Operations         []Operation `json:"operations"`
}

type ConnectionInfo struct {
	MachineID    string   `json:"machineId"`
	Transport    string   `json:"transport"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	MoshHost     string   `json:"moshHost,omitempty"`
	MoshPort     int      `json:"moshPort,omitempty"`
	Username     string   `json:"username"`
	Capabilities []string `json:"capabilities"`
}

type CreateSnapshotInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	RootOnly    bool   `json:"rootOnly,omitempty"`
}

type RestoreMachineInput struct {
	SnapshotID   string `json:"snapshotId,omitempty"`
	DesiredState string `json:"desiredState,omitempty"`
}

type Snapshot struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description,omitempty"`
	OrgID           string       `json:"orgId"`
	SourceMachineID string       `json:"sourceMachineId"`
	Status          string       `json:"status"`
	RootSizeBytes   int64        `json:"rootSizeBytes"`
	DataSizeBytes   int64        `json:"dataSizeBytes,omitempty"`
	DefaultSpec     *MachineSpec `json:"defaultSpec,omitempty"`
	Error           string       `json:"error,omitempty"`
	CreatedAt       string       `json:"createdAt"`
}

type SnapshotList struct {
	Snapshots []Snapshot `json:"snapshots"`
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

type ActivityLease struct {
	ID        string `json:"id"`
	MachineID string `json:"machineId"`
	ExpiresAt string `json:"expiresAt"`
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

func (c *Client) RenameMachine(ctx context.Context, apiKey, machineID, name string) (*Machine, error) {
	return c.UpdateMachine(ctx, apiKey, machineID, UpdateMachineInput{Name: &name})
}

func (c *Client) UpdateMachine(ctx context.Context, apiKey, machineID string, in UpdateMachineInput) (*Machine, error) {
	var out Machine
	_, err := c.doJSON(ctx, http.MethodPatch, "/v1/machines/"+url.PathEscape(machineID), apiKey, in, true, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ExecMachine(ctx context.Context, apiKey, machineID string, in ExecInput) (*ExecResult, error) {
	var out ExecResult
	_, err := c.doWithHTTP(ctx, c.longHTTP, http.MethodPost, "/v1/machines/"+url.PathEscape(machineID)+"/exec", apiKey, in, false, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ReadMachineFiles(ctx context.Context, apiKey, machineID, filePath string) (*MachineFiles, error) {
	p := "/v1/machines/" + url.PathEscape(machineID) + "/files"
	if filePath != "" {
		p += "?path=" + url.QueryEscape(filePath)
	}
	var out MachineFiles
	_, err := c.doWithHTTP(ctx, c.longHTTP, http.MethodGet, p, apiKey, nil, false, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) WriteMachineFile(ctx context.Context, apiKey, machineID, filePath, mode string, parents bool, content []byte) (*FileWritten, error) {
	q := url.Values{}
	q.Set("path", filePath)
	if mode != "" {
		q.Set("mode", mode)
	}
	if parents {
		q.Set("parents", "true")
	}
	reqPath := "/v1/machines/" + url.PathEscape(machineID) + "/files?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+reqPath, bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "instabox-vm-monitor/1.0")

	resp, err := c.longHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeUpstreamError(resp.StatusCode, rawBody)
	}
	var out FileWritten
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &out); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}
	return &out, nil
}

func (c *Client) ListMachinePorts(ctx context.Context, apiKey, machineID string) (*PublicPortList, error) {
	var out PublicPortList
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+url.PathEscape(machineID)+"/ports", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PublishMachinePort(ctx context.Context, apiKey, machineID string, in PublishPortInput) (*PublicPort, error) {
	var out PublicPort
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+url.PathEscape(machineID)+"/ports", apiKey, in, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UnpublishMachinePort(ctx context.Context, apiKey, machineID string, port int) error {
	_, err := c.doJSON(ctx, http.MethodDelete, "/v1/machines/"+url.PathEscape(machineID)+"/ports/"+strconv.Itoa(port), apiKey, nil, false, nil)
	return err
}

func (c *Client) ListMachineProcesses(ctx context.Context, apiKey, machineID string, all bool) (*ProcessList, error) {
	p := "/v1/machines/" + url.PathEscape(machineID) + "/processes"
	if all {
		p += "?all=true"
	}
	var out ProcessList
	_, err := c.doWithHTTP(ctx, c.longHTTP, http.MethodGet, p, apiKey, nil, false, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMachineLogs(ctx context.Context, apiKey, machineID string, tail int, after int64) (*MachineLogs, error) {
	q := url.Values{}
	if tail > 0 {
		q.Set("tail", strconv.Itoa(tail))
	}
	if after > 0 {
		q.Set("after", strconv.FormatInt(after, 10))
	}
	p := "/v1/machines/" + url.PathEscape(machineID) + "/logs"
	if enc := q.Encode(); enc != "" {
		p += "?" + enc
	}
	var out MachineLogs
	_, err := c.doJSON(ctx, http.MethodGet, p, apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMachineConnectionInfo(ctx context.Context, apiKey, machineID string) (*ConnectionInfo, error) {
	var out ConnectionInfo
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/machines/"+url.PathEscape(machineID)+"/connection-info", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListSnapshots(ctx context.Context, apiKey string) ([]Snapshot, error) {
	var out SnapshotList
	_, err := c.doJSON(ctx, http.MethodGet, "/v1/snapshots", apiKey, nil, false, &out)
	if err != nil {
		return nil, err
	}
	return out.Snapshots, nil
}

func (c *Client) CreateSnapshot(ctx context.Context, apiKey, machineID string, in CreateSnapshotInput) (*Snapshot, error) {
	var out Snapshot
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+url.PathEscape(machineID)+"/snapshots", apiKey, in, true, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RestoreMachine(ctx context.Context, apiKey, machineID string, in RestoreMachineInput) (*ActionResult, error) {
	var op Operation
	_, err := c.doJSON(ctx, http.MethodPost, "/v1/machines/"+url.PathEscape(machineID)+"/restore", apiKey, in, true, &op)
	if err != nil {
		return nil, err
	}
	return &ActionResult{Operation: &op}, nil
}

func (c *Client) DeleteSnapshot(ctx context.Context, apiKey, snapshotID, confirmValue string) error {
	if confirmValue == "" {
		confirmValue = snapshotID
	}
	extra := map[string]string{"X-Confirm-Delete": confirmValue}
	_, err := c.doJSONWithHeaders(ctx, http.MethodDelete, "/v1/snapshots/"+url.PathEscape(snapshotID), apiKey, nil, true, extra, nil)
	return err
}

func (c *Client) DeleteMachine(ctx context.Context, apiKey, machineID, confirmValue string) (*ActionResult, error) {
	if confirmValue == "" {
		confirmValue = machineID
	}
	var op Operation
	extra := map[string]string{"X-Confirm-Delete": confirmValue}
	_, err := c.doJSONWithHeaders(ctx, http.MethodDelete, "/v1/machines/"+url.PathEscape(machineID), apiKey, nil, true, extra, &op)
	if err != nil {
		return nil, err
	}
	return &ActionResult{Operation: &op}, nil
}

func (c *Client) PutActivityLease(ctx context.Context, apiKey, machineID, leaseID string, ttlSeconds int) (*ActivityLease, error) {
	if strings.TrimSpace(leaseID) == "" {
		leaseID = "ibmon-keepalive"
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	} else if ttlSeconds < 30 {
		ttlSeconds = 30
	} else if ttlSeconds > 3600 {
		ttlSeconds = 3600
	}
	var out ActivityLease
	payload := struct {
		TTLSeconds int `json:"ttlSeconds"`
	}{TTLSeconds: ttlSeconds}
	path := "/v1/machines/" + url.PathEscape(machineID) + "/activity-leases/" + url.PathEscape(leaseID)
	_, err := c.doJSON(ctx, http.MethodPut, path, apiKey, payload, false, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteActivityLease(ctx context.Context, apiKey, machineID, leaseID string) error {
	if strings.TrimSpace(leaseID) == "" {
		leaseID = "ibmon-keepalive"
	}
	path := "/v1/machines/" + url.PathEscape(machineID) + "/activity-leases/" + url.PathEscape(leaseID)
	_, err := c.doJSON(ctx, http.MethodDelete, path, apiKey, nil, false, nil)
	return err
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

type TerminalOptions struct {
	Cols    int
	Rows    int
	Session string
	Wake    string
	Restart string
}

func (c *Client) dialTarget(u *url.URL) (string, *tls.Config) {
	if c.http != nil && c.http.Transport != nil {
		if ov, ok := c.http.Transport.(interface {
			TerminalDialTarget() (string, *tls.Config)
		}); ok {
			if base, tlsCfg := ov.TerminalDialTarget(); base != "" {
				if bu, err := url.Parse(base); err == nil {
					cp := *u
					switch strings.ToLower(bu.Scheme) {
					case "https":
						cp.Scheme = "wss"
					case "http":
						cp.Scheme = "ws"
					default:
						cp.Scheme = bu.Scheme
					}
					cp.Host = bu.Host
					return cp.String(), tlsCfg
				}
			}
		}
		if tr, ok := c.http.Transport.(*http.Transport); ok && tr.TLSClientConfig != nil {
			return u.String(), tr.TLSClientConfig
		}
	}
	return u.String(), nil
}

func (c *Client) DialTerminal(ctx context.Context, apiKey, machineID string, opts TerminalOptions) (*ws.Conn, *APIError, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, nil, err
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/machines/" + url.PathEscape(machineID) + "/terminal"

	q := url.Values{}
	if opts.Cols >= 10 && opts.Cols <= 1000 {
		q.Set("cols", strconv.Itoa(opts.Cols))
	}
	if opts.Rows >= 4 && opts.Rows <= 500 {
		q.Set("rows", strconv.Itoa(opts.Rows))
	}
	if sess := strings.TrimSpace(opts.Session); sess != "" {
		q.Set("session", sess)
	}
	if opts.Wake == "0" {
		q.Set("wake", "0")
	}
	if opts.Restart == "1" {
		q.Set("restart", "1")
	}
	u.RawQuery = q.Encode()

	hdr := http.Header{}
	hdr.Set("X-API-Key", apiKey)
	hdr.Set("User-Agent", "instabox-vm-monitor/1.0")

	targetURL, tlsCfg := c.dialTarget(u)
	conn, err := ws.DialContext(ctx, targetURL, hdr, nil, tlsCfg)
	if err != nil {
		var he *ws.HandshakeError
		if errors.As(err, &he) {
			apiErr := decodeUpstreamError(he.StatusCode, he.Body)
			return nil, apiErr, apiErr
		}
		return nil, nil, err
	}
	return conn, nil, nil
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
	return c.doJSONWithHeaders(ctx, method, path, apiKey, body, needIdem, nil, out)
}

func (c *Client) doJSONWithHeaders(ctx context.Context, method, path, apiKey string, body any, needIdem bool, extraHeaders map[string]string, out any) (time.Time, error) {
	return c.doWithHTTP(ctx, c.http, method, path, apiKey, body, needIdem, extraHeaders, out)
}

func decodeUpstreamError(status int, rawBody []byte) *APIError {
	if status >= 300 && status < 400 {
		return &APIError{
			Status:  http.StatusBadGateway,
			Code:    "unexpected_redirect",
			Message: fmt.Sprintf("unexpected redirect (%d) from upstream API", status),
		}
	}
	var env struct {
		Error APIErrorBody `json:"error"`
	}
	_ = json.Unmarshal(rawBody, &env)
	apiErr := &APIError{
		Status:            status,
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
		apiErr.Message = http.StatusText(status)
	}
	return apiErr
}

func (c *Client) doWithHTTP(ctx context.Context, hc *http.Client, method, path, apiKey string, body any, needIdem bool, extraHeaders map[string]string, out any) (time.Time, error) {
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
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := hc.Do(req)
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
		return serverTime, decodeUpstreamError(resp.StatusCode, rawBody)
	}

	if out != nil && len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, out); err != nil {
			return serverTime, fmt.Errorf("decode response: %w", err)
		}
	}
	return serverTime, nil
}
