package blocklist

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"netora-dns/pkg/configengine"
	"netora-dns/pkg/model"
	"netora-dns/pkg/policy"
	"netora-dns/pkg/storage"
)

// AutoUpdateStatus contains live telemetry regarding the blocklist auto-update system.
type AutoUpdateStatus struct {
	Enabled        bool           `json:"enabled"`
	IntervalHours  int            `json:"interval_hours"`
	LastSyncTime   *time.Time     `json:"last_sync_time"`
	NextSyncTime   *time.Time     `json:"next_sync_time"`
	IsSyncing      bool           `json:"is_syncing"`
	LastStatus     string         `json:"last_status"`
	LastError      string         `json:"last_error,omitempty"`
	CategoryCounts map[string]int `json:"category_counts"`
	TotalDomains   int            `json:"total_domains"`
}

// FeedDefinition defines standard repository files and fallback upstream mirrors.
type FeedDefinition struct {
	ID           string
	Name         string
	Category     string
	RepoFilename string
	FallbackURLs []string
	LocalPath    string
}

// Manager orchestrates periodic blocklist updates, sanitization, Unbound compilation, and atomic apply.
type Manager struct {
	mu           sync.RWMutex
	syncMu       sync.Mutex
	blocklistDir string
	confDDir     string
	gitHubToken  string
	repoBaseURL  string
	interval     time.Duration
	status       AutoUpdateStatus
	store        storage.Store
	engine       *configengine.Engine
	nodeID       string
	policyEngine *policy.PolicyEngine
	stopCh       chan struct{}
	client       *http.Client
}

// ManagerOptions configures the blocklist manager.
type ManagerOptions struct {
	BlocklistDir string
	ConfDDir     string
	GitHubToken  string
	RepoBaseURL  string
	Interval     time.Duration
	Store        storage.Store
	Engine       *configengine.Engine
	NodeID       string
	PolicyEngine *policy.PolicyEngine
	HTTPClient   *http.Client
}

// StandardCoreFeeds returns the canonical 6 categories managed by Netora DNS.
func StandardCoreFeeds(blocklistDir string) []FeedDefinition {
	return []FeedDefinition{
		{
			ID:           "feed-adult",
			Name:         "Parental Control (18+ Dewasa)",
			Category:     "ADULT",
			RepoFilename: "adult-domains.txt",
			FallbackURLs: []string{
				"https://raw.githubusercontent.com/alexsannikov/adguardhome-filters/refs/heads/master/porn.txt",
				"https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/porn/hosts",
			},
			LocalPath: filepath.Join(blocklistDir, "adult-domains.txt"),
		},
		{
			ID:           "feed-gambling",
			Name:         "Judi Online & Slot Gacor",
			Category:     "GAMBLING",
			RepoFilename: "gambling-domains.txt",
			FallbackURLs: []string{
				"https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/gambling/hosts",
			},
			LocalPath: filepath.Join(blocklistDir, "gambling-domains.txt"),
		},
		{
			ID:           "feed-ads",
			Name:         "Iklan, Pop-up & Pelacak (Global & ABPindo)",
			Category:     "ADS_TRACKING",
			RepoFilename: "ads.txt",
			FallbackURLs: []string{
				"https://raw.githubusercontent.com/ABPindo/indonesianadblockrules/master/subscriptions/domain.txt",
				"https://raw.githubusercontent.com/ABPindo/indonesianadblockrules/master/subscriptions/aghome.txt",
				"https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/popupads.txt",
				"https://adguardteam.github.io/HostlistsRegistry/assets/filter_1.txt",
			},
			LocalPath: filepath.Join(blocklistDir, "ads.txt"),
		},
		{
			ID:           "feed-malware",
			Name:         "Malware, Phishing & Botnet Shield",
			Category:     "MALWARE_ADS",
			RepoFilename: "malware.txt",
			FallbackURLs: []string{
				"https://raw.githubusercontent.com/hagezi/dns-blocklists/main/hosts/light.txt",
				"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
			},
			LocalPath: filepath.Join(blocklistDir, "malware.txt"),
		},
		{
			ID:           "feed-custom",
			Name:         "Custom ISP Blacklist",
			Category:     "CUSTOM",
			RepoFilename: "custom.txt",
			FallbackURLs: []string{
				"https://raw.githubusercontent.com/desienkz-slp/DNS/refs/heads/main/block_mine.txt",
			},
			LocalPath: filepath.Join(blocklistDir, "custom.txt"),
		},
		{
			ID:           "feed-whitelist",
			Name:         "Whitelist Pengecualian",
			Category:     "WHITELIST",
			RepoFilename: "whitelist.txt",
			FallbackURLs: nil,
			LocalPath:    filepath.Join(blocklistDir, "whitelist.txt"),
		},
	}
}

// NewManager initializes the Blocklist Manager.
func NewManager(opts ManagerOptions) *Manager {
	if opts.BlocklistDir == "" {
		opts.BlocklistDir = "/etc/unbound/blocklist"
	}
	if opts.ConfDDir == "" {
		opts.ConfDDir = "/etc/unbound/conf.d"
	}
	if opts.Interval <= 0 {
		opts.Interval = 12 * time.Hour
	}
	token := opts.GitHubToken
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	repoURL := opts.RepoBaseURL
	if repoURL == "" {
		repoURL = "https://raw.githubusercontent.com/desienkz-slp/unbound/main/blocklist/"
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	_ = os.MkdirAll(opts.BlocklistDir, 0755)
	_ = os.MkdirAll(opts.ConfDDir, 0755)

	m := &Manager{
		blocklistDir: opts.BlocklistDir,
		confDDir:     opts.ConfDDir,
		gitHubToken:  token,
		repoBaseURL:  repoURL,
		interval:     opts.Interval,
		store:        opts.Store,
		engine:       opts.Engine,
		nodeID:       opts.NodeID,
		policyEngine: opts.PolicyEngine,
		stopCh:       make(chan struct{}),
		client:       client,
		status: AutoUpdateStatus{
			Enabled:        true,
			IntervalHours:  int(opts.Interval.Hours()),
			LastStatus:     "IDLE",
			CategoryCounts: make(map[string]int),
		},
	}

	return m
}

// ResolveFeeds returns operator-configured feeds from DesiredState, falling back to StandardCoreFeeds.
func (m *Manager) ResolveFeeds(configured []model.FeedConfig) []FeedDefinition {
	if len(configured) == 0 {
		return StandardCoreFeeds(m.blocklistDir)
	}
	feeds := make([]FeedDefinition, 0, len(configured))
	for _, c := range configured {
		if !c.Enabled {
			continue
		}
		feeds = append(feeds, FeedDefinition{
			ID:           c.ID,
			Name:         c.Name,
			Category:     c.Category,
			RepoFilename: c.RepoFilename,
			FallbackURLs: c.FallbackURLs,
			LocalPath:    filepath.Join(m.blocklistDir, c.RepoFilename),
		})
	}
	if len(feeds) == 0 {
		return StandardCoreFeeds(m.blocklistDir)
	}
	return feeds
}

// StartScheduler begins background auto-updating on the configured interval.
func (m *Manager) StartScheduler(ctx context.Context) {
	// 1. Initial delayed sync 15 seconds after startup
	go func() {
		select {
		case <-time.After(15 * time.Second):
			_, _ = m.SyncAll(ctx, "Startup Auto-Update")
		case <-m.stopCh:
			return
		}
	}()

	// 2. Periodic ticker loop
	ticker := time.NewTicker(m.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = m.SyncAll(ctx, "Scheduled Auto-Update")
			case <-m.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop terminates background schedulers.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// GetStatus returns a copy of the current auto-update status.
func (m *Manager) GetStatus() AutoUpdateStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// SyncReport contains detailed results of a synchronization execution.
type SyncReport struct {
	Success        bool           `json:"success"`
	Trigger        string         `json:"trigger"`
	Timestamp      time.Time      `json:"timestamp"`
	DurationMs     int64          `json:"duration_ms"`
	TotalDomains   int            `json:"total_domains"`
	CategoryCounts map[string]int `json:"category_counts"`
	Message        string         `json:"message"`
	Reloaded       bool           `json:"reloaded"`
}

// SyncAll downloads, parses, sanitizes, and applies all blocklists.
func (m *Manager) SyncAll(ctx context.Context, trigger string) (*SyncReport, error) {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()

	start := time.Now()

	m.mu.Lock()
	m.status.IsSyncing = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.status.IsSyncing = false
		m.mu.Unlock()
	}()

	feeds := StandardCoreFeeds(m.blocklistDir)

	// Step 1: Download & parse whitelist first
	whitelistMap := make(map[string]bool)
	var whitelistDomains []string
	for _, feed := range feeds {
		if feed.Category == "WHITELIST" {
			data, _ := m.downloadFeed(ctx, feed)
			whitelistDomains = m.parseCleanDomains(data)
			for _, d := range whitelistDomains {
				whitelistMap[d] = true
			}
			_ = os.WriteFile(feed.LocalPath, []byte(strings.Join(whitelistDomains, "\n")+"\n"), 0644)
			break
		}
	}

	// Also include allowlist records from storage
	if m.store != nil {
		if al, err := m.store.ListAllowlist(ctx); err == nil {
			for _, a := range al {
				whitelistMap[strings.ToLower(strings.TrimSpace(a.Domain))] = true
			}
		}
	}

	// Step 2: Download & sanitize blocklists
	// Feed priority for cross-category dedup (highest first): MALWARE_ADS > ADS_TRACKING > ADULT > GAMBLING > CUSTOM
	// A domain that appears in multiple feeds is only written to the highest-priority feed's conf file,
	// preventing duplicate local-zone warnings in unbound-checkconf.
	feedPriority := []string{"MALWARE_ADS", "ADS_TRACKING", "ADULT", "GAMBLING", "CUSTOM"}
	priorityIndex := func(cat string) int {
		for i, c := range feedPriority {
			if c == cat {
				return i
			}
		}
		return len(feedPriority)
	}

	// Sort feeds by priority before processing
	sortedFeeds := make([]FeedDefinition, 0, len(feeds))
	for _, f := range feeds {
		if f.Category != "WHITELIST" {
			sortedFeeds = append(sortedFeeds, f)
		}
	}
	sort.Slice(sortedFeeds, func(i, j int) bool {
		return priorityIndex(sortedFeeds[i].Category) < priorityIndex(sortedFeeds[j].Category)
	})

	categoryCounts := make(map[string]int)
	categoryCounts["WHITELIST"] = len(whitelistDomains)
	totalDomains := 0
	globalBlockSeen := make(map[string]bool) // cross-feed dedup

	for _, feed := range sortedFeeds {
		data, ok := m.downloadFeed(ctx, feed)
		if !ok && len(data) == 0 {
			// If download failed and local doesn't exist, read disk fallback
			data, _ = os.ReadFile(feed.LocalPath)
		}

		rawDomains := m.parseCleanDomains(data)
		var cleanedDomains []string
		seen := make(map[string]bool)

		for _, d := range rawDomains {
			if !whitelistMap[d] && !seen[d] && !globalBlockSeen[d] {
				seen[d] = true
				globalBlockSeen[d] = true
				cleanedDomains = append(cleanedDomains, d)
			}
		}


		// Write sanitized list back to disk
		if len(cleanedDomains) > 0 {
			_ = os.WriteFile(feed.LocalPath, []byte(strings.Join(cleanedDomains, "\n")+"\n"), 0644)
		}

		count := len(cleanedDomains)
		categoryCounts[feed.Category] = count
		totalDomains += count

		// Update or Insert into storage
		if m.store != nil {
			now := time.Now().UTC()
			rec := &storage.FilterRecord{
				ID:            feed.ID,
				Name:          feed.Name,
				Category:      feed.Category,
				SourceURL:     feed.RepoFilename, // Private filename or neutral tag
				LocalPath:     feed.LocalPath,
				RuleCount:     count,
				Enabled:       true,
				LastUpdatedAt: &now,
				CreatedAt:     now,
			}
			// Check if already exists to preserve Enabled state
			if existingList, err := m.store.ListFilters(ctx); err == nil {
				for _, ex := range existingList {
					if ex.ID == feed.ID {
						rec.Enabled = ex.Enabled
						rec.CreatedAt = ex.CreatedAt
						break
					}
				}
			}
			_ = m.store.UpsertFilter(ctx, rec)
		}

		// Register in PolicyEngine for live diagnosis explainability
		if m.policyEngine != nil {
			for _, d := range cleanedDomains {
				m.policyEngine.AddBlockRule(d, feed.Category, feed.Name, d)
			}
		}
	}

	// Step 3: Trigger Unbound Atomic Apply
	reloaded := false
	if m.engine != nil && m.store != nil {
		cfg, err := m.store.GetDesiredState(ctx, m.nodeID)
		if err == nil && cfg != nil {
			cfg.VersionID = fmt.Sprintf("v-auto-sync-%d", time.Now().Unix())
			if _, applyErr := m.engine.Apply(ctx, cfg, trigger, "Auto-update synced all filter blocklists"); applyErr == nil {
				_ = m.store.SaveDesiredState(ctx, cfg)
				reloaded = true
			}
		}
	}

	now := time.Now().UTC()
	nextSync := now.Add(m.interval)

	m.mu.Lock()
	m.status.LastSyncTime = &now
	m.status.NextSyncTime = &nextSync
	m.status.LastStatus = "SUCCESS"
	m.status.LastError = ""
	m.status.CategoryCounts = categoryCounts
	m.status.TotalDomains = totalDomains
	m.mu.Unlock()

	report := &SyncReport{
		Success:        true,
		Trigger:        trigger,
		Timestamp:      now,
		DurationMs:     time.Since(start).Milliseconds(),
		TotalDomains:   totalDomains,
		CategoryCounts: categoryCounts,
		Reloaded:       reloaded,
		Message: fmt.Sprintf("Berhasil memperbarui seluruh filter: %d total domain terblokir (%d Dewasa, %d Judi, %d Iklan, %d Malware)",
			totalDomains, categoryCounts["ADULT"], categoryCounts["GAMBLING"], categoryCounts["ADS_TRACKING"], categoryCounts["MALWARE_ADS"]),
	}

	return report, nil
}

// downloadFeed tries private repo first, then fallbacks, then local disk.
func (m *Manager) downloadFeed(ctx context.Context, feed FeedDefinition) ([]byte, bool) {
	tryURL := func(u string) ([]byte, bool) {
		if u == "" {
			return nil, false
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, false
		}
		if m.gitHubToken != "" && strings.Contains(u, "github") {
			req.Header.Set("Authorization", "token "+m.gitHubToken)
		}
		resp, err := m.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if len(data) > 0 {
				return data, true
			}
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil, false
	}

	// 1. Try private repository raw URL
	repoURL := m.repoBaseURL + feed.RepoFilename
	if data, ok := tryURL(repoURL); ok {
		return data, true
	}

	// 2. Try fallbacks
	for _, fallback := range feed.FallbackURLs {
		if data, ok := tryURL(fallback); ok {
			return data, true
		}
	}

	// 3. Fallback to reading disk
	if data, err := os.ReadFile(feed.LocalPath); err == nil && len(data) > 0 {
		return data, false
	}

	return nil, false
}

// parseCleanDomains sanitizes raw lines into valid domain strings.
func (m *Manager) parseCleanDomains(content []byte) []string {
	var domains []string
	lines := strings.Split(string(content), "\n")
	seen := make(map[string]bool)
	for _, line := range lines {
		d := configengine.CleanDomain(line)
		if d != "" && !seen[d] {
			seen[d] = true
			domains = append(domains, d)
		}
	}
	return domains
}
