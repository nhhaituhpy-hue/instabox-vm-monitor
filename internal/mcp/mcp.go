package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/crypto"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/instabox"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/monitor"
	"github.com/nhhaituhpy-hue/instabox-vm-monitor/internal/store"
)

type Server struct {
	store  *store.Store
	poller *monitor.Poller
	tools  []toolDef
}

func NewHandler(st *store.Store, poller *monitor.Poller) http.Handler {
	s := &Server{
		store:  st,
		poller: poller,
	}
	s.tools = buildToolDefs()
	return s
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type toolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

type toolDef struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema map[string]any   `json:"inputSchema"`
	Annotations *toolAnnotations `json:"annotations,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodGet:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	case http.MethodPost:
	default:
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeRPCError(w, http.StatusBadRequest, json.RawMessage("null"), -32700, "failed to read request body")
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeRPCError(w, http.StatusBadRequest, json.RawMessage("null"), -32700, "invalid JSON-RPC payload")
		return
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		writeRPCError(w, http.StatusBadRequest, req.ID, -32600, "jsonrpc must be 2.0")
		return
	}

	// JSON-RPC notifications carry no id (or null id for non-requests)
	if len(req.ID) == 0 || string(req.ID) == "null" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	sid := strings.TrimSpace(r.Header.Get("MCP-Session-Id"))
	switch req.Method {
	case "initialize":
		var initParams struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &initParams)
		proto := negotiateProtocol(initParams.ProtocolVersion)
		if sid == "" {
			sid = crypto.NewUUID()
		}
		w.Header().Set("MCP-Session-Id", sid)
		w.Header().Set("MCP-Protocol-Version", proto)
		writeRPCResult(w, req.ID, map[string]any{
			"protocolVersion": proto,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "instabox-vm-monitor",
				"version": "1.0.0",
			},
			"instructions": "Full control plane for saved instabox.sh accounts and VMs. When deleting a machine via delete_machine, you MUST first ask the user for explicit confirmation with the machine name and ID before supplying the confirmationToken.",
		})
		return

	case "ping":
		if sid != "" {
			w.Header().Set("MCP-Session-Id", sid)
		}
		writeRPCResult(w, req.ID, map[string]any{})
		return

	case "tools/list":
		if sid != "" {
			w.Header().Set("MCP-Session-Id", sid)
		}
		writeRPCResult(w, req.ID, map[string]any{
			"tools": s.tools,
		})
		return

	case "tools/call":
		if sid != "" {
			w.Header().Set("MCP-Session-Id", sid)
		}
		var callParams struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil || strings.TrimSpace(callParams.Name) == "" {
			writeRPCError(w, http.StatusOK, req.ID, -32602, "invalid tools/call params")
			return
		}
		res, isErr := s.dispatchTool(r.Context(), callParams.Name, callParams.Arguments)
		rawText, _ := json.MarshalIndent(res, "", "  ")
		writeRPCResult(w, req.ID, map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": string(rawText),
				},
			},
			"isError": isErr,
		})
		return

	default:
		writeRPCError(w, http.StatusOK, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func negotiateProtocol(clientVer string) string {
	switch strings.TrimSpace(clientVer) {
	case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
		return strings.TrimSpace(clientVer)
	default:
		return "2025-06-18"
	}
}

func writeRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func writeRPCError(w http.ResponseWriter, httpStatus int, id json.RawMessage, code int, msg string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	})
}

func errPayload(err error) map[string]any {
	var apiErr *instabox.APIError
	if errors.As(err, &apiErr) {
		return map[string]any{
			"error":             apiErr.Message,
			"code":              apiErr.Code,
			"status":            apiErr.Status,
			"retryable":         apiErr.Retryable,
			"retryAfterSeconds": apiErr.RetryAfterSeconds,
			"reason":            apiErr.Reason,
			"hint":              apiErr.Hint,
		}
	}
	return map[string]any{"error": err.Error()}
}

func (s *Server) resolveAccountMachine(ctx context.Context, accountID, machineID string) (string, store.MachineRecord, error) {
	accountID = strings.TrimSpace(accountID)
	machineID = strings.TrimSpace(machineID)
	if machineID == "" {
		return "", store.MachineRecord{}, errors.New("machineId is required")
	}
	if accountID != "" {
		m, err := s.store.GetMachine(ctx, accountID, machineID)
		if err != nil {
			return "", store.MachineRecord{}, err
		}
		return accountID, *m, nil
	}
	all, err := s.store.ListMachines(ctx)
	if err != nil {
		return "", store.MachineRecord{}, err
	}
	var matches []store.MachineRecord
	for _, m := range all {
		if m.MachineID == machineID {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		// Also match by exact unique machine name as convenience if unambiguous
		for _, m := range all {
			if m.Name == machineID {
				matches = append(matches, m)
			}
		}
	}
	if len(matches) == 0 {
		return "", store.MachineRecord{}, store.ErrMachineNotFound
	}
	if len(matches) > 1 {
		return "", store.MachineRecord{}, fmt.Errorf("ambiguous machine %q across multiple accounts; pass accountId explicitly", machineID)
	}
	return matches[0].AccountID, matches[0], nil
}

func (s *Server) waitForOperation(ctx context.Context, accountID, opID string, waitSeconds int) (*instabox.Operation, error) {
	if opID == "" || waitSeconds <= 0 {
		return nil, nil
	}
	if waitSeconds > 120 {
		waitSeconds = 120
	}
	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	var last *instabox.Operation
	for time.Now().Before(deadline) {
		rem := time.Until(deadline)
		step := 15 * time.Second
		if rem < step {
			step = rem
		}
		waitStr := strconv.Itoa(max(1, int(step.Seconds()))) + "s"
		op, err := s.poller.GetOperation(ctx, accountID, opID, waitStr)
		if err != nil {
			return last, err
		}
		last = op
		if op != nil && op.State != "queued" && op.State != "running" {
			_ = s.poller.SyncAccount(ctx, accountID)
			return op, nil
		}
	}
	return last, nil
}

func (s *Server) dispatchTool(ctx context.Context, name string, rawArgs json.RawMessage) (any, bool) {
	if len(rawArgs) == 0 {
		rawArgs = json.RawMessage("{}")
	}
	switch name {
	case "list_fleet":
		var in struct {
			AccountID string `json:"accountId"`
			State     string `json:"state"`
			Query     string `json:"query"`
			Refresh   bool   `json:"refresh"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		if in.Refresh {
			if in.AccountID != "" {
				_ = s.poller.SyncAccount(ctx, in.AccountID)
			} else {
				s.poller.SyncAll(ctx)
			}
		}
		accounts, err := s.store.ListAccounts(ctx)
		if err != nil {
			return errPayload(err), true
		}
		machines, err := s.store.ListMachines(ctx)
		if err != nil {
			return errPayload(err), true
		}
		q := strings.ToLower(strings.TrimSpace(in.Query))
		filtered := make([]store.MachineRecord, 0, len(machines))
		for _, m := range machines {
			if in.AccountID != "" && m.AccountID != in.AccountID {
				continue
			}
			if in.State != "" && m.State != in.State {
				continue
			}
			if q != "" {
				hay := strings.ToLower(m.Name + " " + m.MachineID + " " + m.TemplateID + " " + m.AccountAlias)
				if !strings.Contains(hay, q) {
					continue
				}
			}
			filtered = append(filtered, m)
		}
		return map[string]any{
			"accounts": accounts,
			"machines": filtered,
		}, false

	case "get_account_options":
		var in struct {
			AccountID string `json:"accountId"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		if strings.TrimSpace(in.AccountID) != "" {
			opts, err := s.poller.GetAccountMachineOptions(ctx, strings.TrimSpace(in.AccountID))
			if err != nil {
				return errPayload(err), true
			}
			return opts, false
		}
		accounts, err := s.store.ListAccounts(ctx)
		if err != nil {
			return errPayload(err), true
		}
		machines, _ := s.store.ListMachines(ctx)
		counts := make(map[string]int)
		for _, m := range machines {
			counts[m.AccountID]++
		}
		summaries := make([]map[string]any, 0, len(accounts))
		for _, a := range accounts {
			summaries = append(summaries, map[string]any{
				"accountId":     a.ID,
				"alias":         a.Alias,
				"orgTier":       a.OrgTier,
				"enabled":       a.Enabled,
				"machineCount":  counts[a.ID],
				"instaboxLogin": a.InstaboxLogin,
				"orgUsage":      a.OrgUsage,
			})
		}
		return map[string]any{"accounts": summaries}, false

	case "create_machine":
		var in struct {
			AccountID               string            `json:"accountId"`
			Name                    string            `json:"name"`
			TemplateID              string            `json:"templateId"`
			SnapshotID              string            `json:"snapshotId"`
			VCPUs                   int               `json:"vcpus"`
			MemMiB                  int               `json:"memMiB"`
			DesiredState            string            `json:"desiredState"`
			AutoSuspendAfterSeconds *int              `json:"autoSuspendAfterSeconds"`
			MaxRunMinutes           *int              `json:"maxRunMinutes"`
			DeleteAfterSeconds      int               `json:"deleteAfterSeconds"`
			ArchiveSeconds          int               `json:"archiveSeconds"`
			Env                     map[string]string `json:"env"`
			WaitSeconds             int               `json:"waitSeconds"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		in.AccountID = strings.TrimSpace(in.AccountID)
		if in.AccountID == "" {
			return errPayload(errors.New("accountId is required")), true
		}
		if in.DeleteAfterSeconds != 0 && (in.DeleteAfterSeconds < 60 || in.DeleteAfterSeconds > 86400) {
			return errPayload(errors.New("deleteAfterSeconds must be 0 or 60-86400")), true
		}
		if in.ArchiveSeconds != 0 && (in.ArchiveSeconds < 60 || in.ArchiveSeconds > 86400) {
			return errPayload(errors.New("archiveSeconds must be 0 or 60-86400")), true
		}
		res, err := s.poller.CreateMachine(ctx, in.AccountID, instabox.CreateMachineInput{
			Name:                    strings.TrimSpace(in.Name),
			TemplateID:              strings.TrimSpace(in.TemplateID),
			SnapshotID:              strings.TrimSpace(in.SnapshotID),
			VCPUs:                   in.VCPUs,
			MemMiB:                  in.MemMiB,
			DesiredState:            strings.TrimSpace(in.DesiredState),
			AutoSuspendAfterSeconds: in.AutoSuspendAfterSeconds,
			MaxRunMinutes:           in.MaxRunMinutes,
			DeleteAfterSeconds:      in.DeleteAfterSeconds,
			ArchiveSeconds:          in.ArchiveSeconds,
			Env:                     in.Env,
		})
		if err != nil {
			return errPayload(err), true
		}
		if in.WaitSeconds > 0 && res.Operation != nil && res.Operation.ID != "" && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			if op, wErr := s.waitForOperation(ctx, in.AccountID, res.Operation.ID, in.WaitSeconds); wErr == nil && op != nil {
				res.Operation = op
			}
		}
		var created *store.MachineRecord
		if res.Operation != nil && res.Operation.MachineID != "" {
			created, _ = s.store.GetMachine(ctx, in.AccountID, res.Operation.MachineID)
		}
		return map[string]any{"operation": res.Operation, "machine": created}, false

	case "control_machine":
		var in struct {
			AccountID               string  `json:"accountId"`
			MachineID               string  `json:"machineId"`
			Action                  string  `json:"action"`
			Name                    *string `json:"name"`
			AutoSuspendAfterSeconds *int    `json:"autoSuspendAfterSeconds"`
			MaxRunMinutes           *int    `json:"maxRunMinutes"`
			VCPUs                   int     `json:"vcpus"`
			MemMiB                  int     `json:"memMiB"`
			CancelScheduled         bool    `json:"cancelScheduled"`
			Apply                   string  `json:"apply"`
			Keepalive               *bool   `json:"keepalive"`
			Schedule                *bool   `json:"schedule"`
			ScheduleWindows         string  `json:"scheduleWindows"`
			ScheduleTZOffsetMin     int     `json:"scheduleTzOffsetMin"`
			WaitSeconds             int     `json:"waitSeconds"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		out := map[string]any{"accountId": accID, "machineId": mRec.MachineID}

		if in.Name != nil || in.AutoSuspendAfterSeconds != nil || in.MaxRunMinutes != nil || in.VCPUs != 0 || in.MemMiB != 0 || in.CancelScheduled || in.Apply != "" {
			upd, uErr := s.poller.UpdateMachine(ctx, accID, mRec.MachineID, instabox.UpdateMachineInput{
				Name:                    in.Name,
				VCPUs:                   in.VCPUs,
				MemMiB:                  in.MemMiB,
				AutoSuspendAfterSeconds: in.AutoSuspendAfterSeconds,
				MaxRunMinutes:           in.MaxRunMinutes,
				CancelScheduled:         in.CancelScheduled,
				Apply:                   in.Apply,
			})
			if uErr != nil {
				return errPayload(uErr), true
			}
			out["machine"] = upd
		}

		if in.Keepalive != nil {
			ka, kErr := s.poller.SetMachineKeepalive(ctx, accID, mRec.MachineID, *in.Keepalive)
			if kErr != nil {
				return errPayload(kErr), true
			}
			out["machine"] = ka
		}

		if in.Schedule != nil {
			windows := in.ScheduleWindows
			if windows == "" {
				windows = mRec.ScheduleWindows
			}
			sch, sErr := s.poller.SetMachineSchedule(ctx, accID, mRec.MachineID, *in.Schedule, windows, in.ScheduleTZOffsetMin)
			if sErr != nil {
				return errPayload(sErr), true
			}
			out["machine"] = sch
		}

		if act := strings.TrimSpace(in.Action); act != "" {
			var action instabox.Action
			switch instabox.Action(act) {
			case instabox.ActionWake, instabox.ActionSuspend, instabox.ActionStart, instabox.ActionStop, instabox.ActionResume:
				action = instabox.Action(act)
			default:
				return errPayload(fmt.Errorf("unsupported action %q; allowed: start, stop, suspend, wake, resume", act)), true
			}
			res, aErr := s.poller.ExecuteAction(ctx, accID, mRec.MachineID, action)
			if aErr != nil {
				return errPayload(aErr), true
			}
			if in.WaitSeconds > 0 && res.Operation != nil && res.Operation.ID != "" && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
				if op, wErr := s.waitForOperation(ctx, accID, res.Operation.ID, in.WaitSeconds); wErr == nil && op != nil {
					res.Operation = op
				}
			}
			out["actionResult"] = res
			if fresh, gErr := s.store.GetMachine(ctx, accID, mRec.MachineID); gErr == nil {
				out["machine"] = fresh
			}
		}
		if _, has := out["machine"]; !has {
			if fresh, gErr := s.store.GetMachine(ctx, accID, mRec.MachineID); gErr == nil {
				out["machine"] = fresh
			}
		}
		return out, false

	case "fork_machine":
		var in struct {
			AccountID               string            `json:"accountId"`
			MachineID               string            `json:"machineId"`
			Name                    string            `json:"name"`
			VCPUs                   int               `json:"vcpus"`
			MemMiB                  int               `json:"memMiB"`
			AutoSuspendAfterSeconds *int              `json:"autoSuspendAfterSeconds"`
			MaxRunMinutes           *int              `json:"maxRunMinutes"`
			DeleteAfterSeconds      int               `json:"deleteAfterSeconds"`
			Metadata                map[string]string `json:"metadata"`
			WaitSeconds             int               `json:"waitSeconds"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		if in.DeleteAfterSeconds != 0 && (in.DeleteAfterSeconds < 60 || in.DeleteAfterSeconds > 86400) {
			return errPayload(errors.New("deleteAfterSeconds must be 0 or 60-86400")), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		res, err := s.poller.ForkMachine(ctx, accID, mRec.MachineID, instabox.ForkMachineInput{
			Name:                    strings.TrimSpace(in.Name),
			VCPUs:                   in.VCPUs,
			MemMiB:                  in.MemMiB,
			AutoSuspendAfterSeconds: in.AutoSuspendAfterSeconds,
			MaxRunMinutes:           in.MaxRunMinutes,
			DeleteAfterSeconds:      in.DeleteAfterSeconds,
			Metadata:                in.Metadata,
		})
		if err != nil {
			return errPayload(err), true
		}
		if in.WaitSeconds > 0 && res.Operation != nil && res.Operation.ID != "" && res.Operation.State != "succeeded" && res.Operation.State != "failed" {
			if op, wErr := s.waitForOperation(ctx, accID, res.Operation.ID, in.WaitSeconds); wErr == nil && op != nil {
				res.Operation = op
			}
		}
		var forked *store.MachineRecord
		if res.Operation != nil && res.Operation.MachineID != "" {
			forked, _ = s.store.GetMachine(ctx, accID, res.Operation.MachineID)
		}
		return map[string]any{"operation": res.Operation, "machine": forked}, false

	case "delete_machine":
		var in struct {
			AccountID         string `json:"accountId"`
			MachineID         string `json:"machineId"`
			ConfirmationToken string `json:"confirmationToken"`
			ConfirmName       string `json:"confirmName"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		if strings.TrimSpace(in.ConfirmationToken) == "" {
			ch, cErr := s.poller.CreateDeleteChallenge(ctx, accID, mRec.MachineID)
			if cErr != nil {
				return errPayload(cErr), true
			}
			return ch, false
		}
		if strings.TrimSpace(in.ConfirmName) != "" && strings.TrimSpace(in.ConfirmName) != mRec.Name && strings.TrimSpace(in.ConfirmName) != mRec.MachineID {
			return errPayload(fmt.Errorf("confirmName %q does not match machine name %q", in.ConfirmName, mRec.Name)), true
		}
		if err := s.poller.ConsumeDeleteChallenge(accID, mRec.MachineID, in.ConfirmationToken); err != nil {
			return errPayload(err), true
		}
		res, err := s.poller.DeleteMachine(ctx, accID, mRec.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		return map[string]any{
			"deleted":   true,
			"accountId": accID,
			"machineId": mRec.MachineID,
			"operation": res.Operation,
		}, false

	case "exec_command":
		var in struct {
			AccountID      string            `json:"accountId"`
			MachineID      string            `json:"machineId"`
			Cmd            string            `json:"cmd"`
			Argv           []string          `json:"argv"`
			Cwd            string            `json:"cwd"`
			Envs           map[string]string `json:"envs"`
			Stdin          string            `json:"stdin"`
			TimeoutSeconds int               `json:"timeoutSeconds"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		if in.Cmd == "" && len(in.Argv) == 0 {
			return errPayload(errors.New("either cmd or argv is required")), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		res, err := s.poller.ExecMachine(ctx, accID, mRec.MachineID, instabox.ExecInput{
			Cmd:            in.Cmd,
			Argv:           in.Argv,
			Cwd:            in.Cwd,
			Envs:           in.Envs,
			Stdin:          in.Stdin,
			TimeoutSeconds: in.TimeoutSeconds,
		})
		if err != nil {
			return errPayload(err), true
		}
		return res, false

	case "read_files":
		var in struct {
			AccountID string `json:"accountId"`
			MachineID string `json:"machineId"`
			Path      string `json:"path"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		res, err := s.poller.ReadMachineFiles(ctx, accID, mRec.MachineID, in.Path)
		if err != nil {
			return errPayload(err), true
		}
		return res, false

	case "write_file":
		var in struct {
			AccountID     string `json:"accountId"`
			MachineID     string `json:"machineId"`
			Path          string `json:"path"`
			Content       string `json:"content"`
			ContentBase64 string `json:"contentBase64"`
			Mode          string `json:"mode"`
			Parents       bool   `json:"parents"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		if strings.TrimSpace(in.Path) == "" {
			return errPayload(errors.New("path is required")), true
		}
		var data []byte
		if in.ContentBase64 != "" {
			var decErr error
			data, decErr = base64.StdEncoding.DecodeString(in.ContentBase64)
			if decErr != nil {
				return errPayload(fmt.Errorf("invalid contentBase64: %w", decErr)), true
			}
		} else {
			data = []byte(in.Content)
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		res, err := s.poller.WriteMachineFile(ctx, accID, mRec.MachineID, in.Path, in.Mode, in.Parents, data)
		if err != nil {
			return errPayload(err), true
		}
		return res, false

	case "manage_ports":
		var in struct {
			AccountID   string `json:"accountId"`
			MachineID   string `json:"machineId"`
			Action      string `json:"action"`
			Port        int    `json:"port"`
			Name        string `json:"name"`
			Access      string `json:"access"`
			Protocol    string `json:"protocol"`
			RotateToken bool   `json:"rotateToken"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		switch strings.ToLower(strings.TrimSpace(in.Action)) {
		case "", "list":
			res, err := s.poller.ListMachinePorts(ctx, accID, mRec.MachineID)
			if err != nil {
				return errPayload(err), true
			}
			return res, false
		case "publish":
			if in.Port < 1024 || in.Port > 65535 {
				return errPayload(errors.New("port must be between 1024 and 65535")), true
			}
			res, err := s.poller.PublishMachinePort(ctx, accID, mRec.MachineID, instabox.PublishPortInput{
				Port:        in.Port,
				Name:        in.Name,
				Access:      in.Access,
				Protocol:    in.Protocol,
				RotateToken: in.RotateToken,
			})
			if err != nil {
				return errPayload(err), true
			}
			return res, false
		case "unpublish":
			if in.Port < 1024 || in.Port > 65535 {
				return errPayload(errors.New("port must be between 1024 and 65535")), true
			}
			if err := s.poller.UnpublishMachinePort(ctx, accID, mRec.MachineID, in.Port); err != nil {
				return errPayload(err), true
			}
			return map[string]any{"unpublished": true, "port": in.Port}, false
		default:
			return errPayload(fmt.Errorf("unsupported manage_ports action %q", in.Action)), true
		}

	case "inspect_machine":
		var in struct {
			AccountID string `json:"accountId"`
			MachineID string `json:"machineId"`
			Kind      string `json:"kind"`
			All       bool   `json:"all"`
			Tail      int    `json:"tail"`
			After     int64  `json:"after"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
		if err != nil {
			return errPayload(err), true
		}
		switch strings.ToLower(strings.TrimSpace(in.Kind)) {
		case "processes":
			res, err := s.poller.ListMachineProcesses(ctx, accID, mRec.MachineID, in.All)
			if err != nil {
				return errPayload(err), true
			}
			return res, false
		case "logs":
			res, err := s.poller.GetMachineLogs(ctx, accID, mRec.MachineID, in.Tail, in.After)
			if err != nil {
				return errPayload(err), true
			}
			return res, false
		case "connection_info", "ssh":
			res, err := s.poller.GetMachineConnectionInfo(ctx, accID, mRec.MachineID)
			if err != nil {
				return errPayload(err), true
			}
			return res, false
		default:
			return errPayload(fmt.Errorf("unsupported inspect_machine kind %q; allowed: processes, logs, connection_info", in.Kind)), true
		}

	case "manage_snapshots":
		var in struct {
			AccountID    string `json:"accountId"`
			Action       string `json:"action"`
			MachineID    string `json:"machineId"`
			SnapshotID   string `json:"snapshotId"`
			Name         string `json:"name"`
			Description  string `json:"description"`
			RootOnly     bool   `json:"rootOnly"`
			DesiredState string `json:"desiredState"`
			ConfirmName  string `json:"confirmName"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		switch strings.ToLower(strings.TrimSpace(in.Action)) {
		case "", "list":
			if strings.TrimSpace(in.AccountID) == "" {
				return errPayload(errors.New("accountId is required to list snapshots")), true
			}
			snaps, err := s.poller.ListSnapshots(ctx, strings.TrimSpace(in.AccountID))
			if err != nil {
				return errPayload(err), true
			}
			return map[string]any{"snapshots": snaps}, false
		case "create":
			accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
			if err != nil {
				return errPayload(err), true
			}
			if strings.TrimSpace(in.Name) == "" {
				return errPayload(errors.New("name is required to create snapshot")), true
			}
			sn, err := s.poller.CreateSnapshot(ctx, accID, mRec.MachineID, instabox.CreateSnapshotInput{
				Name:        strings.TrimSpace(in.Name),
				Description: in.Description,
				RootOnly:    in.RootOnly,
			})
			if err != nil {
				return errPayload(err), true
			}
			return sn, false
		case "restore":
			accID, mRec, err := s.resolveAccountMachine(ctx, in.AccountID, in.MachineID)
			if err != nil {
				return errPayload(err), true
			}
			res, err := s.poller.RestoreMachine(ctx, accID, mRec.MachineID, instabox.RestoreMachineInput{
				SnapshotID:   strings.TrimSpace(in.SnapshotID),
				DesiredState: strings.TrimSpace(in.DesiredState),
			})
			if err != nil {
				return errPayload(err), true
			}
			return res, false
		case "delete":
			accID := strings.TrimSpace(in.AccountID)
			if accID == "" || strings.TrimSpace(in.SnapshotID) == "" {
				return errPayload(errors.New("accountId and snapshotId are required to delete snapshot")), true
			}
			confirm := strings.TrimSpace(in.ConfirmName)
			if confirm == "" {
				confirm = strings.TrimSpace(in.SnapshotID)
			}
			if err := s.poller.DeleteSnapshot(ctx, accID, strings.TrimSpace(in.SnapshotID), confirm); err != nil {
				return errPayload(err), true
			}
			return map[string]any{"deleted": true, "snapshotId": in.SnapshotID}, false
		default:
			return errPayload(fmt.Errorf("unsupported manage_snapshots action %q", in.Action)), true
		}

	case "get_operation":
		var in struct {
			AccountID   string `json:"accountId"`
			OperationID string `json:"operationId"`
			Wait        string `json:"wait"`
		}
		if err := json.Unmarshal(rawArgs, &in); err != nil {
			return errPayload(err), true
		}
		if strings.TrimSpace(in.AccountID) == "" || strings.TrimSpace(in.OperationID) == "" {
			return errPayload(errors.New("accountId and operationId are required")), true
		}
		op, err := s.poller.GetOperation(ctx, strings.TrimSpace(in.AccountID), strings.TrimSpace(in.OperationID), strings.TrimSpace(in.Wait))
		if err != nil {
			return errPayload(err), true
		}
		return op, false

	default:
		return errPayload(fmt.Errorf("unknown tool %q", name)), true
	}
}

func buildToolDefs() []toolDef {
	return []toolDef{
		{
			Name:        "list_fleet",
			Description: "List all saved instabox.sh accounts and their virtual machines (optionally filtering by accountId, state, or search query, or triggering a live sync).",
			Annotations: &toolAnnotations{ReadOnlyHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"accountId": map[string]any{"type": "string", "description": "Optional account ID filter"},
					"state":     map[string]any{"type": "string", "enum": []string{"running", "suspended", "stopped"}, "description": "Optional VM state filter"},
					"query":     map[string]any{"type": "string", "description": "Optional search substring matching VM name, ID, template, or account alias"},
					"refresh":   map[string]any{"type": "boolean", "description": "Set true to force live sync with instabox.sh before listing"},
				},
			},
		},
		{
			Name:        "get_account_options",
			Description: "Inspect account tier, org usage limits, allowed specs, and creatable templates (omit accountId to summarize all accounts and find available capacity).",
			Annotations: &toolAnnotations{ReadOnlyHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"accountId": map[string]any{"type": "string", "description": "Optional account ID to inspect available templates and specs"},
				},
			},
		},
		{
			Name:        "create_machine",
			Description: "Create a new instabox.sh virtual machine on a chosen saved account from a template or snapshot.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"accountId"},
				"properties": map[string]any{
					"accountId":               map[string]any{"type": "string"},
					"name":                    map[string]any{"type": "string", "description": "Optional lowercase DNS-safe name (^[a-z0-9][a-z0-9-]{0,62}$)"},
					"templateId":              map[string]any{"type": "string", "description": "Template ID (e.g. agent-browser, android, omarchy, ci)"},
					"snapshotId":              map[string]any{"type": "string", "description": "Optional snapshot ID instead of templateId"},
					"vcpus":                   map[string]any{"type": "integer"},
					"memMiB":                  map[string]any{"type": "integer"},
					"desiredState":            map[string]any{"type": "string", "enum": []string{"running", "stopped"}},
					"autoSuspendAfterSeconds": map[string]any{"type": "integer", "description": "Paid plans: whole minutes 300-1800s or 0=never; free plan: fixed 1800"},
					"maxRunMinutes":           map[string]any{"type": "integer", "description": "-1 unlimited, positive minutes caps each run"},
					"deleteAfterSeconds":      map[string]any{"type": "integer", "description": "Hard self-delete deadline 60-86400s; replaces the retired timeout/onTimeout lifetime fields"},
					"archiveSeconds":          map[string]any{"type": "integer", "description": "Stop the VM after 60-86400s, files kept"},
					"env":                     map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
					"waitSeconds":             map[string]any{"type": "integer", "description": "Optional seconds (up to 120) to wait for create operation completion"},
				},
			},
		},
		{
			Name:        "control_machine",
			Description: "Control a VM lifecycle (start, stop, suspend, wake, resume), rename or update idle/max-run settings, or toggle the 24/7 Keep-awake service.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId"},
				"properties": map[string]any{
					"accountId":               map[string]any{"type": "string", "description": "Optional when machineId is unique across accounts"},
					"machineId":               map[string]any{"type": "string"},
					"action":                  map[string]any{"type": "string", "enum": []string{"start", "stop", "suspend", "wake", "resume"}},
					"name":                    map[string]any{"type": "string", "description": "New lowercase machine name"},
					"autoSuspendAfterSeconds": map[string]any{"type": "integer", "description": "Paid: whole minutes 300-1800s or 0=never; free: fixed 1800"},
					"maxRunMinutes":           map[string]any{"type": "integer", "description": "-1 unlimited; positive minutes caps each run even while busy"},
					"vcpus":                   map[string]any{"type": "integer", "description": "Resize a stopped VM (with memMiB); one of the account's allowedSpecs"},
					"memMiB":                  map[string]any{"type": "integer", "description": "New memory in MiB of a stopped VM (with vcpus)"},
					"cancelScheduled":         map[string]any{"type": "boolean", "description": "Clear a stop/delete scheduled for a macOS VM lease end"},
					"apply":                   map[string]any{"type": "string", "enum": []string{"restart"}, "description": "resize a running/paused VM by stop+resize+start in one operation"},
					"keepalive":               map[string]any{"type": "boolean", "description": "Enable/disable automatic keep-awake & 8h power-cycle service"},
					"schedule":                map[string]any{"type": "boolean", "description": "Enable/disable time-window scheduling (mutually exclusive with keepalive)"},
					"scheduleWindows":         map[string]any{"type": "string", "description": `Daily run windows, e.g. "06:00-08:00, 11:00-12:00, 18:00-19:00" — machine is auto-started inside windows and stopped outside`},
					"scheduleTzOffsetMin":     map[string]any{"type": "integer", "description": "Fixed UTC offset in minutes for the windows (default 420 = UTC+7)"},
					"waitSeconds":             map[string]any{"type": "integer", "description": "Optional seconds (up to 120) to wait for lifecycle operation"},
				},
			},
		},
		{
			Name:        "fork_machine",
			Description: "Copy a running or stopped VM's disks into a new machine in seconds (instabox.sh fork). Processes and memory are not copied; the fork boots fresh. Useful for parallel agent experiments.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId"},
				"properties": map[string]any{
					"accountId":               map[string]any{"type": "string"},
					"machineId":               map[string]any{"type": "string"},
					"name":                    map[string]any{"type": "string", "description": "Name of the fork; empty = <source>-fork-<hex4>"},
					"vcpus":                   map[string]any{"type": "integer"},
					"memMiB":                  map[string]any{"type": "integer"},
					"autoSuspendAfterSeconds": map[string]any{"type": "integer"},
					"maxRunMinutes":           map[string]any{"type": "integer"},
					"deleteAfterSeconds":      map[string]any{"type": "integer", "description": "Hard self-delete deadline 60-86400s for the fork"},
					"metadata":                map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
					"waitSeconds":             map[string]any{"type": "integer", "description": "Optional seconds (up to 120) to wait for fork operation completion"},
				},
			},
		},
		{
			Name:        "delete_machine",
			Description: "DESTRUCTIVE: Permanently delete a VM and its disks (saved snapshots are kept). ALWAYS ask the user for explicit confirmation naming the machine before deleting. Call first without confirmationToken to get a confirmation challenge, ask the user in chat, and ONLY supply confirmationToken once the user confirms.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId"},
				"properties": map[string]any{
					"accountId":         map[string]any{"type": "string"},
					"machineId":         map[string]any{"type": "string"},
					"confirmationToken": map[string]any{"type": "string", "description": "Single-use confirmationToken returned by the first challenge call; ONLY pass after explicit user approval"},
					"confirmName":       map[string]any{"type": "string", "description": "Exact name of the machine being deleted"},
				},
			},
		},
		{
			Name:        "exec_command",
			Description: "Execute a shell command (cmd) or direct program arguments (argv) inside a running or auto-idle VM via instabox.sh /exec.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId"},
				"properties": map[string]any{
					"accountId":      map[string]any{"type": "string"},
					"machineId":      map[string]any{"type": "string"},
					"cmd":            map[string]any{"type": "string", "description": "Shell command executed with bash -lc"},
					"argv":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Program and args executed without shell"},
					"cwd":            map[string]any{"type": "string"},
					"envs":           map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
					"stdin":          map[string]any{"type": "string"},
					"timeoutSeconds": map[string]any{"type": "integer", "description": "Default 60, max 600"},
				},
			},
		},
		{
			Name:        "read_files",
			Description: "List a directory or read a UTF-8 file inside a VM (wakes idle-paused VMs automatically).",
			Annotations: &toolAnnotations{ReadOnlyHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId"},
				"properties": map[string]any{
					"accountId": map[string]any{"type": "string"},
					"machineId": map[string]any{"type": "string"},
					"path":      map[string]any{"type": "string", "description": "Absolute path inside VM (default /)"},
				},
			},
		},
		{
			Name:        "write_file",
			Description: "Write UTF-8 text or base64 binary content to a file inside a VM (optionally creating parent directories and setting mode).",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId", "path"},
				"properties": map[string]any{
					"accountId":     map[string]any{"type": "string"},
					"machineId":     map[string]any{"type": "string"},
					"path":          map[string]any{"type": "string"},
					"content":       map[string]any{"type": "string"},
					"contentBase64": map[string]any{"type": "string"},
					"mode":          map[string]any{"type": "string", "description": "Octal file mode e.g. 0755 or 0644"},
					"parents":       map[string]any{"type": "boolean", "description": "Create missing parent directories"},
				},
			},
		},
		{
			Name:        "manage_ports",
			Description: "List, publish (public or private HTTPS/TCP), or unpublish exposed ports on a VM.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId", "action"},
				"properties": map[string]any{
					"accountId":   map[string]any{"type": "string"},
					"machineId":   map[string]any{"type": "string"},
					"action":      map[string]any{"type": "string", "enum": []string{"list", "publish", "unpublish"}},
					"port":        map[string]any{"type": "integer"},
					"name":        map[string]any{"type": "string"},
					"access":      map[string]any{"type": "string", "enum": []string{"public", "private"}},
					"protocol":    map[string]any{"type": "string", "enum": []string{"http", "tcp"}},
					"rotateToken": map[string]any{"type": "boolean"},
				},
			},
		},
		{
			Name:        "inspect_machine",
			Description: "Inspect a VM's running processes, serial console & operation logs, or SSH connection info.",
			Annotations: &toolAnnotations{ReadOnlyHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"machineId", "kind"},
				"properties": map[string]any{
					"accountId": map[string]any{"type": "string"},
					"machineId": map[string]any{"type": "string"},
					"kind":      map[string]any{"type": "string", "enum": []string{"processes", "logs", "connection_info"}},
					"all":       map[string]any{"type": "boolean"},
					"tail":      map[string]any{"type": "integer"},
					"after":     map[string]any{"type": "integer"},
				},
			},
		},
		{
			Name:        "manage_snapshots",
			Description: "List snapshots on an account, create a backup snapshot of a VM, restore a VM from a snapshot, or delete a snapshot.",
			Annotations: &toolAnnotations{DestructiveHint: boolPtr(false)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"action"},
				"properties": map[string]any{
					"accountId":    map[string]any{"type": "string"},
					"action":       map[string]any{"type": "string", "enum": []string{"list", "create", "restore", "delete"}},
					"machineId":    map[string]any{"type": "string"},
					"snapshotId":   map[string]any{"type": "string"},
					"name":         map[string]any{"type": "string"},
					"description":  map[string]any{"type": "string"},
					"rootOnly":     map[string]any{"type": "boolean"},
					"desiredState": map[string]any{"type": "string", "enum": []string{"running", "stopped"}},
					"confirmName":  map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "get_operation",
			Description: "Check or long-poll an asynchronous instabox.sh operation ID.",
			Annotations: &toolAnnotations{ReadOnlyHint: boolPtr(true)},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"accountId", "operationId"},
				"properties": map[string]any{
					"accountId":   map[string]any{"type": "string"},
					"operationId": map[string]any{"type": "string"},
					"wait":        map[string]any{"type": "string", "description": "Optional wait duration e.g. 15s"},
				},
			},
		},
	}
}
