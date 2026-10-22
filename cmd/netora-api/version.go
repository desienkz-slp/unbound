package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Candidate locations for the deployed version metadata (first readable wins).
var versionFileCandidates = []string{
	"/var/www/netora-ui/version.json",
	"/opt/netora-dns/current/version.json",
	"/var/lib/netora-dns/version.json",
	"version.json",
	"web/src/version.json",
}

const (
	updateStateFile = "/var/lib/netora-dns/update-state.json"
	githubTokenFile = "/etc/netora-dns/github.token"
	upstreamRepoURL = "https://github.com/desienkz-slp/unbound.git"
	remoteCheckTTL  = 4 * time.Second
)

// readVersionInfo is the single source of truth for version metadata.
// Order: built-in fallback < version.json < update-state.json (live deployment state).
func readVersionInfo(nodeID string) map[string]interface{} {
	info := map[string]interface{}{
		"version":      "v1.0.48",
		"commit_count": 48,
		"build":        48,
		"git_commit":   "bc2988f",
		"git_branch":   "main",
		"channel":      "production",
		"node_id":      nodeID,
		"status":       "SUCCESS",
		"build_time":   time.Now().UTC().Format(time.RFC3339),
	}

	for _, cand := range versionFileCandidates {
		data, err := os.ReadFile(cand)
		if err != nil {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			continue
		}
		for k, v := range parsed {
			info[k] = v
		}
		break
	}

	data, err := os.ReadFile(updateStateFile)
	if err != nil {
		return info
	}
	var st struct {
		CurrentRevision string `json:"current_revision"`
		Channel         string `json:"channel"`
		UpdateTime      string `json:"update_time"`
		UpdateStatus    string `json:"update_status"`
		LastError       string `json:"last_error"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return info
	}
	if st.CurrentRevision != "" {
		info["git_commit"] = st.CurrentRevision
		if len(st.CurrentRevision) >= 7 {
			info["short_commit"] = st.CurrentRevision[:7]
		}
	}
	if st.Channel != "" {
		info["channel"] = st.Channel
	}
	if st.UpdateTime != "" {
		info["update_time"] = st.UpdateTime
	}
	if st.UpdateStatus != "" {
		info["last_status"] = st.UpdateStatus
	}
	if st.LastError != "" && st.LastError != "none" {
		info["last_error"] = st.LastError
	}
	return info
}

// Handler: System Version (/api/v1/system/version)
func (s *Server) handleSystemVersion(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, readVersionInfo(s.nodeID))
}

// Handler: System Update Status (/api/v1/system/update/status)
func (s *Server) handleSystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	s.updateMu.Lock()
	isUpdating, lastErr, lastTime := s.isUpdating, s.lastUpdateErr, s.lastUpdateTime
	s.updateMu.Unlock()

	info := readVersionInfo(s.nodeID)
	info["is_updating"] = isUpdating
	info["update_available"] = false
	if !lastTime.IsZero() {
		info["last_update_time"] = lastTime.Format(time.RFC3339)
	}
	if ut, ok := info["update_time"].(string); ok && ut != "" {
		info["last_update_time"] = ut
	}
	if _, ok := info["last_error"]; !ok {
		info["last_error"] = lastErr
	}

	// Fast remote check via git ls-remote if requested
	if r.URL.Query().Get("check_remote") == "true" {
		if remoteSHA := fetchRemoteHead(r.Context()); remoteSHA != "" {
			info["remote_commit"] = remoteSHA
			curr, _ := info["git_commit"].(string)
			if curr != "" && !strings.HasPrefix(remoteSHA, curr) && !strings.HasPrefix(curr, remoteSHA) {
				info["update_available"] = true
			}
		}
	}

	s.writeJSON(w, http.StatusOK, info)
}

// fetchRemoteHead returns the SHA of refs/heads/main on the upstream repo, or "" on failure.
func fetchRemoteHead(parent context.Context) string {
	ctx, cancel := context.WithTimeout(parent, remoteCheckTTL)
	defer cancel()
	repoURL := buildAuthenticatedRepoURL(upstreamRepoURL, githubTokenFile)
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repoURL, "refs/heads/main")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	if fields := strings.Fields(string(out)); len(fields) > 0 {
		return fields[0]
	}
	return ""
}
