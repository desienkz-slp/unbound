package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"netora-dns/pkg/auth"
	"netora-dns/pkg/storage"
)

// Handler: Policies
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		policies, err := s.store.ListPolicies(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, policies)
		return
	}
	if r.Method == http.MethodPost {
		var p storage.PolicyRecord
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if p.ID == "" {
			p.ID = fmt.Sprintf("pol-%d", time.Now().UnixNano())
		}
		if err := s.store.UpsertPolicy(r.Context(), &p); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, p)
		return
	}
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Filters
func (s *Server) handleFilters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		filters, err := s.store.ListFilters(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if filters == nil {
			filters = []storage.FilterRecord{}
		}
		canonicalFeeds := []storage.FilterRecord{
			{
				ID:        "feed-adult",
				Name:      "Parental Control (18+ Dewasa)",
				Category:  "ADULT",
				SourceURL: "adult-domains.txt",
				LocalPath: "/etc/unbound/blocklist/adult-domains.txt",
				RuleCount: 46466,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-gambling",
				Name:      "Judi Online & Slot Gacor",
				Category:  "GAMBLING",
				SourceURL: "gambling-domains.txt",
				LocalPath: "/etc/unbound/blocklist/gambling-domains.txt",
				RuleCount: 83549,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-ads",
				Name:      "Iklan, Pop-up & Pelacak",
				Category:  "ADS_TRACKING",
				SourceURL: "ads.txt",
				LocalPath: "/etc/unbound/blocklist/ads.txt",
				RuleCount: 52100,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-malware",
				Name:      "Malware, Phishing & Botnet Shield",
				Category:  "MALWARE_ADS",
				SourceURL: "malware.txt",
				LocalPath: "/etc/unbound/blocklist/malware.txt",
				RuleCount: 71800,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-custom",
				Name:      "Custom ISP Blacklist",
				Category:  "CUSTOM",
				SourceURL: "custom.txt",
				LocalPath: "/etc/unbound/blocklist/custom.txt",
				RuleCount: 15,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-hagezi",
				Name:      "Hagezi Multi-PRO Threat Feed",
				Category:  "MALWARE_ADS",
				SourceURL: "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/pro.txt",
				LocalPath: "/var/lib/netora/filters/hagezi-pro.txt",
				RuleCount: 142300,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-adguard",
				Name:      "AdGuard Base DNS Filter",
				Category:  "ADS_TRACKING",
				SourceURL: "https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt",
				LocalPath: "/var/lib/netora/filters/adguard.txt",
				RuleCount: 48900,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-abpindo",
				Name:      "ABPindo — Filter Iklan & Pelacak Indonesia",
				Category:  "ADS_TRACKING",
				SourceURL: "https://raw.githubusercontent.com/ABPindo/indonesianadblockrules/master/subscriptions/aghome.txt",
				LocalPath: "/etc/unbound/blocklist/abpindo.txt",
				RuleCount: 140,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
			{
				ID:        "feed-stevenblack",
				Name:      "StevenBlack Unified Hosts File",
				Category:  "HOSTS_BLOCK",
				SourceURL: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
				LocalPath: "/etc/unbound/blocklists/stevenblack.hosts",
				RuleCount: 73200,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			},
		}

		existingIDs := make(map[string]bool)
		for _, f := range filters {
			existingIDs[f.ID] = true
		}

		hasNew := false
		for i := range canonicalFeeds {
			if !existingIDs[canonicalFeeds[i].ID] {
				_ = s.store.UpsertFilter(r.Context(), &canonicalFeeds[i])
				filters = append(filters, canonicalFeeds[i])
				hasNew = true
			}
		}
		if hasNew {
			if refetched, err := s.store.ListFilters(r.Context()); err == nil {
				filters = refetched
			}
		}
		s.writeJSON(w, http.StatusOK, filters)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Name      string `json:"name"`
			Category  string `json:"category"`
			SourceURL string `json:"source_url"`
			LocalPath string `json:"local_path"`
			Enabled   *bool  `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" || (req.SourceURL == "" && req.LocalPath == "") {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and source_url or local_path are required"})
			return
		}
		id := fmt.Sprintf("filter-%d", time.Now().UnixNano())
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		f := &storage.FilterRecord{
			ID:        id,
			Name:      req.Name,
			Category:  req.Category,
			SourceURL: req.SourceURL,
			LocalPath: req.LocalPath,
			RuleCount: 0,
			Enabled:   enabled,
			CreatedAt: time.Now().UTC(),
		}
		if err := s.store.UpsertFilter(r.Context(), f); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, f)
		return
	}
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Filter Subroutes (/api/v1/filters/{id})
func (s *Server) handleFilterSubroutes(w http.ResponseWriter, r *http.Request) {
	subPath := strings.TrimPrefix(r.URL.Path, "/api/v1/filters/")
	parts := strings.Split(subPath, "/")
	id := parts[0]
	if id == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	if r.Method == http.MethodDelete {
		if err := s.store.DeleteFilter(r.Context(), id); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED", "id": id})
		return
	}

	if r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodPost {
		filters, err := s.store.ListFilters(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		var target *storage.FilterRecord
		for _, f := range filters {
			if f.ID == id {
				cp := f
				target = &cp
				break
			}
		}

		var req struct {
			Enabled   *bool   `json:"enabled"`
			Name      *string `json:"name"`
			Category  *string `json:"category"`
			SourceURL *string `json:"source_url"`
			LocalPath *string `json:"local_path"`
			RuleCount *int    `json:"rule_count"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if target == nil {
			target = &storage.FilterRecord{
				ID:        id,
				Name:      id,
				Category:  "BLOCKLIST",
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			}
		}

		if req.Enabled != nil {
			target.Enabled = *req.Enabled
		} else if len(parts) > 1 && parts[1] == "toggle" {
			target.Enabled = !target.Enabled
		}

		if req.Name != nil && *req.Name != "" {
			target.Name = *req.Name
		}
		if req.Category != nil && *req.Category != "" {
			target.Category = *req.Category
		}
		if req.SourceURL != nil {
			target.SourceURL = *req.SourceURL
		}
		if req.LocalPath != nil {
			target.LocalPath = *req.LocalPath
		}
		if req.RuleCount != nil {
			target.RuleCount = *req.RuleCount
		}

		now := time.Now().UTC()
		target.LastUpdatedAt = &now

		if err := s.store.UpsertFilter(r.Context(), target); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Dynamically re-apply filter configuration to Unbound
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			s.reapplyFilterState(bgCtx, "Filter Toggled: "+target.Name)
		}()

		s.writeJSON(w, http.StatusOK, target)
		return
	}

	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Sync All Blocklists (/api/v1/blocklists/sync)
func (s *Server) handleSyncBlocklists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	actor := "Manual NOC Trigger"
	if claims, ok := r.Context().Value(userClaimsKey).(*auth.Claims); ok && claims != nil {
		actor = claims.Username
	}

	report, err := s.blocklistManager.SyncAll(r.Context(), actor)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	adultCount := report.CategoryCounts["ADULT"]
	gamblingCount := report.CategoryCounts["GAMBLING"]

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":                report.Success,
		"adult_domains_count":    adultCount,
		"gambling_domains_count": gamblingCount,
		"category_counts":        report.CategoryCounts,
		"total_domains":          report.TotalDomains,
		"duration_ms":            report.DurationMs,
		"timestamp":              report.Timestamp,
		"reloaded":               report.Reloaded,
		"message":                report.Message,
	})
}

// Handler: Auto-Update Status (/api/v1/blocklists/auto-update-status)
func (s *Server) handleBlocklistAutoUpdateStatus(w http.ResponseWriter, r *http.Request) {
	status := s.blocklistManager.GetStatus()
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) reapplyFilterState(ctx context.Context, reason string) {
	if s.engine == nil || s.store == nil {
		return
	}
	cfg, err := s.store.GetDesiredState(ctx, s.nodeID)
	if err != nil || cfg == nil {
		return
	}

	filters, err := s.store.ListFilters(ctx)
	if err == nil {
		for _, f := range filters {
			switch f.ID {
			case "feed-adult":
				cfg.ParentalControl.Enabled = f.Enabled
				cfg.ParentalControl.BlockAdult = f.Enabled
			case "feed-gambling":
				cfg.GamblingBlock.Enabled = f.Enabled
			case "feed-ads", "feed-adguard", "feed-abpindo":
				cfg.AdBlock.Enabled = f.Enabled
			case "feed-malware", "feed-hagezi":
				cfg.MalwareBlock.Enabled = f.Enabled
			case "feed-custom":
				cfg.CustomBlock.Enabled = f.Enabled
			}
		}
	}

	cfg.VersionID = fmt.Sprintf("v-filter-toggle-%d", time.Now().Unix())
	if _, err := s.engine.Apply(ctx, cfg, "NOC Admin", reason); err == nil {
		_ = s.store.SaveDesiredState(ctx, cfg)
	}
}

// Handler: Allowlist
func (s *Server) handleAllowlist(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		al, err := s.store.ListAllowlist(r.Context())
		if err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if al == nil {
			al = []storage.AllowlistRecord{}
		}
		s.writeJSON(w, http.StatusOK, al)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Domain   string `json:"domain"`
			Category string `json:"category"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		claims := r.Context().Value(userClaimsKey).(*auth.Claims)
		if err := s.store.AddAllowlist(r.Context(), req.Domain, req.Category, req.Reason, claims.Username); err != nil {
			s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "ADDED", "domain": req.Domain})
		return
	}
	s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

// Handler: Allowlist Delete
func (s *Server) handleAllowlistDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	domain := strings.TrimPrefix(r.URL.Path, "/api/v1/allowlist/")
	if domain == "" {
		s.writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}
	if err := s.store.RemoveAllowlist(r.Context(), domain); err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED", "domain": domain})
}
