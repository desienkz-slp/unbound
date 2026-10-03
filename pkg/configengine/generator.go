package configengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"netora-dns/pkg/model"
)

// RenderedFile represents a single generated configuration file.
type RenderedFile struct {
	RelativePath string
	Content      []byte
	Checksum     string
}

// RenderResult contains all generated configuration artifacts and metadata.
type RenderResult struct {
	VersionID    string
	NodeID       string
	GeneratedAt  time.Time
	Files        []RenderedFile
	MainChecksum string
}

// GeneratorContext holds prepared data for template evaluation.
type GeneratorContext struct {
	*model.DesiredState
	ConfigDir              string
	GeneratedAt            string
	HasIPv6                bool
	HasTLSUpstream         bool
	ActiveListeners        []model.Listener
	ActiveClientNetworks   []model.ClientNetwork
	ActiveUpstreams        []model.UpstreamServer
	InsecureDomains        []string
	AdultBlockedDomains    []string
	GamblingBlockedDomains []string
	AdsBlockedDomains      []string
	MalwareBlockedDomains  []string
	CustomBlockedDomains   []string
	WhitelistedDomains     []string
	IsGamblingBlockActive  bool
	GamblingRedirectIP     string
	IsAdBlockActive        bool
	AdRedirectIP           string
	IsMalwareBlockActive   bool
	MalwareRedirectIP      string
	IsCustomBlockActive    bool
	CustomRedirectIP       string
	// SupportsServeExpiredAdvanced indicates the installed Unbound is >= 1.9.0,
	// where serve-expired-client-timeout / serve-expired-reply-ttl are valid keywords.
	SupportsServeExpiredAdvanced bool
}

// DefaultAdultBlockedDomains provides curated high-confidence adult/pornographic domains.
var DefaultAdultBlockedDomains = []string{
	"pornhub.com",
	"xhamster.com",
	"xnxx.com",
	"xvideos.com",
	"redtube.com",
	"youporn.com",
	"chaturbate.com",
	"onlyfans.com",
	"stripchat.com",
	"livejasmin.com",
	"cam4.com",
	"beeg.com",
	"spankbang.com",
	"tubegalore.com",
	"eporner.com",
	"tnaflix.com",
	"tube8.com",
	"brazzers.com",
	"drtuber.com",
	"adultfriendfinder.com",
}

// DefaultGamblingBlockedDomains provides curated high-confidence Indonesian and global online gambling / slot / betting domains.
var DefaultGamblingBlockedDomains = []string{
	"sbobet.com",
	"maxbet.com",
	"bet365.com",
	"1xbet.com",
	"betfair.com",
	"stake.com",
	"888casino.com",
	"williamhill.com",
	"bovada.lv",
	"slot88.com",
	"slot777.com",
	"pragmaticplay.com",
	"spadegaming.com",
	"pgsoft.com",
	"joker123.net",
	"habanero.com",
	"judionline.com",
	"taruhan.com",
	"poker88.com",
	"domino99.com",
	"bandarqq.com",
	"zeuslot.com",
	"gacor88.com",
	"rajagacor.com",
	"dewaslot.com",
	"dewaslot99.com",
	"mpo777.com",
	"mpo888.com",
	"ligaciputra.com",
	"asia99.com",
	"idnplay.com",
	"cmd368.com",
	"dafabet.com",
	"m88.com",
	"fun88.com",
	"w88.com",
}

// DefaultAdsBlockedDomains provides curated high-confidence advertising & telemetry domains.
var DefaultAdsBlockedDomains = []string{
	"doubleclick.net",
	"googleads.g.doubleclick.net",
	"pagead2.googlesyndication.com",
	"adservice.google.com",
	"adnxs.com",
	"criteo.com",
	"outbrain.com",
	"taboola.com",
	"popads.net",
	"popcash.net",
	"propellerads.com",
	"exoclick.com",
	"trafficjunky.com",
}

// DefaultMalwareBlockedDomains provides curated high-confidence malware, phishing, and C2 domains.
var DefaultMalwareBlockedDomains = []string{
	"coinhive.com",
	"crypto-loot.com",
	"minr.pw",
	"redline-stealer.com",
	"lumma-c2.cc",
	"asyncrat-c2.net",
	"njrat-dns.org",
	"remcos-dns.net",
	"qakbot-c2.com",
	"emotet-c2.net",
}

// DefaultCustomBlockedDomains provides sample ISP-specific blacklist domains.
var DefaultCustomBlockedDomains = []string{
	"camcaps.ac",
	"cin.ac",
	"rphang.ac",
	"aptilon.com",
	"189cube.com",
}

// DefaultWhitelistedDomains provides common safe domains to always allow.
var DefaultWhitelistedDomains = []string{
	"wa.me",
	"bankmandiri.co.id",
	"bca.co.id",
}

// CleanDomain sanitizes and extracts the bare domain name from ABP, hosts, or raw formats.
func CleanDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "!") {
		return ""
	}
	if strings.HasPrefix(raw, "@@||") {
		raw = raw[4:]
	} else if strings.HasPrefix(raw, "||") {
		raw = raw[2:]
	}
	if idx := strings.IndexAny(raw, "^$/"); idx != -1 {
		raw = raw[:idx]
	}

	parts := strings.Fields(raw)
	if len(parts) >= 2 && (parts[0] == "0.0.0.0" || parts[0] == "127.0.0.1") {
		raw = parts[1]
	} else if len(parts) >= 1 {
		raw = parts[0]
	}

	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimSuffix(raw, ".")

	switch raw {
	case "", "localhost", "localhost.localdomain", "local", "broadcasthost",
		"0.0.0.0", "127.0.0.1", "255.255.255.255", "ip6-localhost", "ip6-loopback", "::1":
		return ""
	}

	if !strings.Contains(raw, ".") {
		return ""
	}

	return raw
}

// hasWildcardPrefix reports whether a DNS Rewrite domain starts with "*."
// (i.e., the user wants a catch-all wildcard redirect zone in Unbound).
func hasWildcardPrefix(domain string) bool {
	return strings.HasPrefix(domain, "*.")
}

// trimWildcardPrefix removes the leading "*." from a wildcard domain name,
// returning the bare zone name suitable for local-zone and local-data directives.
func trimWildcardPrefix(domain string) string {
	return strings.TrimPrefix(domain, "*.")
}

// loadDomainList reads domains from disk files (if existing) or falls back to defaults.
func loadDomainList(filePath string, fallback []string) []string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fallback
	}
	var domains []string
	lines := strings.Split(string(content), "\n")
	seen := make(map[string]bool)
	for _, line := range lines {
		domain := CleanDomain(line)
		if domain != "" && !seen[domain] {
			seen[domain] = true
			domains = append(domains, domain)
		}
	}
	if len(domains) == 0 {
		return fallback
	}
	return domains
}

func filterOutWhitelisted(domains []string, whitelist map[string]bool) []string {
	if len(whitelist) == 0 {
		return domains
	}
	var filtered []string
	for _, d := range domains {
		if !whitelist[d] {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// deduplicateBlocklists eliminates cross-category duplicate domains so each domain
// only appears in exactly one Unbound conf file, preventing duplicate local-zone warnings.
// Priority order (highest first): malware > ads > adult > gambling > custom.
// A domain already claimed by a higher-priority list is stripped from all lower ones.
func deduplicateBlocklists(malware, ads, adult, gambling, custom []string) ([]string, []string, []string, []string, []string) {
	globalSeen := make(map[string]bool)

	dedup := func(list []string) []string {
		var out []string
		for _, d := range list {
			if !globalSeen[d] {
				globalSeen[d] = true
				out = append(out, d)
			}
		}
		return out
	}

	malware = dedup(malware)
	ads = dedup(ads)
	adult = dedup(adult)
	gambling = dedup(gambling)
	custom = dedup(custom)
	return malware, ads, adult, gambling, custom
}

// Generator transforms DesiredState into deterministic Unbound configuration files.
type Generator struct {
	mainTmpl       *template.Template
	insecureTmpl   *template.Template
	recordsTmpl    *template.Template
	safesearchTmpl *template.Template
	parentalTmpl   *template.Template
	gamblingTmpl   *template.Template
	adTmpl         *template.Template
	malwareTmpl    *template.Template
	customTmpl     *template.Template
	whitelistTmpl  *template.Template
}

// NewGenerator initializes and compiles the configuration templates.
func NewGenerator() (*Generator, error) {
	main, err := template.New("main").Parse(MainUnboundConfTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse main unbound template: %w", err)
	}

	insecure, err := template.New("insecure").Parse(DomainInsecureTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse insecure domains template: %w", err)
	}

	wildcardFuncs := template.FuncMap{
		"hasWildcardPrefix":  hasWildcardPrefix,
		"trimWildcardPrefix": trimWildcardPrefix,
	}
	records, err := template.New("records").Funcs(wildcardFuncs).Parse(LocalRecordsTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse local records template: %w", err)
	}

	safesearch, err := template.New("safesearch").Parse(SafeSearchTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse safesearch template: %w", err)
	}

	parental, err := template.New("parental").Parse(ParentalControlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse parental control template: %w", err)
	}

	gambling, err := template.New("gambling").Parse(GamblingFilterTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gambling filter template: %w", err)
	}

	ad, err := template.New("ad").Parse(AdBlockTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse adblock template: %w", err)
	}

	malware, err := template.New("malware").Parse(MalwareBlockTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse malware template: %w", err)
	}

	custom, err := template.New("custom").Parse(CustomBlockTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse custom block template: %w", err)
	}

	whitelist, err := template.New("whitelist").Parse(WhitelistTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse whitelist template: %w", err)
	}

	return &Generator{
		mainTmpl:       main,
		insecureTmpl:   insecure,
		recordsTmpl:    records,
		safesearchTmpl: safesearch,
		parentalTmpl:   parental,
		gamblingTmpl:   gambling,
		adTmpl:         ad,
		malwareTmpl:    malware,
		customTmpl:     custom,
		whitelistTmpl:  whitelist,
	}, nil
}

// unboundVersionRe extracts a semantic version (e.g. 1.17.1) from `unbound -V` output.
var unboundVersionRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

var (
	serveExpiredAdvancedOnce sync.Once
	serveExpiredAdvancedOK   bool
)

// unboundSupportsServeExpiredAdvanced reports whether the locally installed Unbound
// is >= 1.10.0, the release that introduced serve-expired-client-timeout and
// serve-expired-reply-ttl. The result is detected once and cached for the process
// lifetime. On any detection failure it conservatively returns false so that
// unbound-checkconf never rejects the generated configuration on older distros.
func unboundSupportsServeExpiredAdvanced() bool {
	serveExpiredAdvancedOnce.Do(func() {
		serveExpiredAdvancedOK = detectServeExpiredAdvanced()
	})
	return serveExpiredAdvancedOK
}

func detectServeExpiredAdvanced() bool {
	for _, bin := range []string{"unbound", "/usr/sbin/unbound"} {
		out, err := exec.Command(bin, "-V").CombinedOutput()
		if err != nil {
			continue
		}
		if major, minor, ok := parseUnboundVersion(string(out)); ok {
			return major > 1 || (major == 1 && minor >= 10)
		}
	}
	return false
}

// parseUnboundVersion returns the major and minor components of the first
// semantic version found in the provided text.
func parseUnboundVersion(text string) (major, minor int, ok bool) {
	m := unboundVersionRe.FindStringSubmatch(text)
	if len(m) < 3 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(m[1])
	min, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}

// Generate renders the full set of configuration files from the desired state.
func (g *Generator) Generate(state *model.DesiredState, configDir string) (*RenderResult, error) {
	if errs := model.ValidateDesiredState(state); len(errs) > 0 {
		return nil, fmt.Errorf("cannot generate configuration: %v", errs[0])
	}

	isGamblingActive := state.GamblingBlock.Enabled || state.ParentalControl.BlockGambling
	gamblingRedirect := state.GamblingBlock.RedirectIP
	if gamblingRedirect == "" && state.ParentalControl.BlockGambling {
		gamblingRedirect = state.ParentalControl.RedirectIP
	}

	loadWithFallback := func(filename string, def []string) []string {
		p1 := filepath.Join(configDir, "blocklist", filename)
		p2 := filepath.Join("/etc/unbound/blocklist", filename)
		if _, err := os.Stat(p1); err == nil {
			return loadDomainList(p1, def)
		}
		return loadDomainList(p2, def)
	}

	whitelistList := loadWithFallback("whitelist.txt", DefaultWhitelistedDomains)
	wlMap := make(map[string]bool)
	for _, w := range whitelistList {
		wlMap[w] = true
	}

	adultList := filterOutWhitelisted(loadWithFallback("adult-domains.txt", DefaultAdultBlockedDomains), wlMap)
	gamblingList := filterOutWhitelisted(loadWithFallback("gambling-domains.txt", DefaultGamblingBlockedDomains), wlMap)
	adsList := filterOutWhitelisted(loadWithFallback("ads.txt", DefaultAdsBlockedDomains), wlMap)
	malwareList := filterOutWhitelisted(loadWithFallback("malware.txt", DefaultMalwareBlockedDomains), wlMap)
	customList := filterOutWhitelisted(loadWithFallback("custom.txt", DefaultCustomBlockedDomains), wlMap)

	// Cross-category deduplication: each domain only in one conf file (highest priority wins).
	// Eliminates duplicate local-zone warnings in unbound-checkconf.
	malwareList, adsList, adultList, gamblingList, customList = deduplicateBlocklists(
		malwareList, adsList, adultList, gamblingList, customList,
	)

	// Prepare sorted, deterministic collections
	ctx := &GeneratorContext{
		DesiredState:                 state,
		ConfigDir:                    configDir,
		GeneratedAt:                  time.Now().UTC().Format(time.RFC3339),
		AdultBlockedDomains:          adultList,
		GamblingBlockedDomains:       gamblingList,
		AdsBlockedDomains:            adsList,
		MalwareBlockedDomains:        malwareList,
		CustomBlockedDomains:         customList,
		WhitelistedDomains:           whitelistList,
		IsGamblingBlockActive:        isGamblingActive,
		GamblingRedirectIP:           gamblingRedirect,
		IsAdBlockActive:              state.AdBlock.Enabled,
		AdRedirectIP:                 state.AdBlock.RedirectIP,
		IsMalwareBlockActive:         state.MalwareBlock.Enabled,
		MalwareRedirectIP:            state.MalwareBlock.RedirectIP,
		IsCustomBlockActive:          state.CustomBlock.Enabled,
		CustomRedirectIP:             state.CustomBlock.RedirectIP,
		SupportsServeExpiredAdvanced: unboundSupportsServeExpiredAdvanced(),
	}

	// Filter and sort active listeners
	hasWildcardV4 := false
	for _, l := range state.Listeners {
		if l.Enabled && !l.IsIPv6 && !model.IsIPv6Address(l.IP) && l.IP == "0.0.0.0" && l.Port == 53 {
			hasWildcardV4 = true
			break
		}
	}

	for _, l := range state.Listeners {
		if l.Enabled {
			if l.IsIPv6 || model.IsIPv6Address(l.IP) {
				ctx.HasIPv6 = true
				ctx.ActiveListeners = append(ctx.ActiveListeners, l)
			} else {
				// In Linux sockets, binding 0.0.0.0:53 along with 127.0.0.1:53 results in
				// 'Address already in use for 0.0.0.0 port 53'. Loopback is already covered by 0.0.0.0.
				if hasWildcardV4 && l.IP == "127.0.0.1" && l.Port == 53 {
					continue
				}
				ctx.ActiveListeners = append(ctx.ActiveListeners, l)
			}
		}
	}
	sort.Slice(ctx.ActiveListeners, func(i, j int) bool {
		if ctx.ActiveListeners[i].IP != ctx.ActiveListeners[j].IP {
			return ctx.ActiveListeners[i].IP < ctx.ActiveListeners[j].IP
		}
		return ctx.ActiveListeners[i].Port < ctx.ActiveListeners[j].Port
	})

	// Filter and sort active client networks
	for _, netw := range state.ClientNetworks {
		if netw.Enabled {
			ctx.ActiveClientNetworks = append(ctx.ActiveClientNetworks, netw)
		}
	}
	sort.Slice(ctx.ActiveClientNetworks, func(i, j int) bool {
		return ctx.ActiveClientNetworks[i].CIDR < ctx.ActiveClientNetworks[j].CIDR
	})

	// Collect active upstreams in exact user-configured priority order
	for _, up := range state.Upstreams {
		if up.Enabled {
			ctx.ActiveUpstreams = append(ctx.ActiveUpstreams, up)
			if up.Type == model.UpstreamDoT || up.Type == model.UpstreamDoH || up.Port == 853 || up.TLSName != "" {
				ctx.HasTLSUpstream = true
			}
		}
	}

	// Deduplicate and sort insecure domains
	domainSet := make(map[string]struct{})
	for _, d := range state.DNSSEC.InsecureDomains {
		cleaned := strings.TrimSpace(strings.ToLower(d))
		if cleaned != "" {
			domainSet[cleaned] = struct{}{}
		}
	}
	for d := range domainSet {
		ctx.InsecureDomains = append(ctx.InsecureDomains, d)
	}
	sort.Strings(ctx.InsecureDomains)

	// Sort local records
	sort.Slice(ctx.LocalRecords, func(i, j int) bool {
		if ctx.LocalRecords[i].Domain != ctx.LocalRecords[j].Domain {
			return ctx.LocalRecords[i].Domain < ctx.LocalRecords[j].Domain
		}
		return ctx.LocalRecords[i].Type < ctx.LocalRecords[j].Type
	})

	result := &RenderResult{
		VersionID:   state.VersionID,
		NodeID:      state.NodeID,
		GeneratedAt: time.Now().UTC(),
	}

	// 1. Render unbound.conf
	mainBuf := new(bytes.Buffer)
	if err := g.mainTmpl.Execute(mainBuf, ctx); err != nil {
		return nil, fmt.Errorf("failed rendering unbound.conf: %w", err)
	}
	mainBytes := mainBuf.Bytes()
	mainSum := calculateSHA256(mainBytes)
	result.MainChecksum = mainSum
	result.Files = append(result.Files, RenderedFile{
		RelativePath: "unbound.conf",
		Content:      mainBytes,
		Checksum:     mainSum,
	})

	// 2. Render conf.d/domain-insecure.conf (if any)
	if len(ctx.InsecureDomains) > 0 {
		insecureBuf := new(bytes.Buffer)
		if err := g.insecureTmpl.Execute(insecureBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering domain-insecure.conf: %w", err)
		}
		insecureBytes := insecureBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/domain-insecure.conf",
			Content:      insecureBytes,
			Checksum:     calculateSHA256(insecureBytes),
		})
	} else {
		emptyBytes := []byte("# No domain-insecure exceptions configured\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/domain-insecure.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 3. Render conf.d/local-records.conf
	allRecords := make([]model.LocalRecord, len(ctx.LocalRecords))
	copy(allRecords, ctx.LocalRecords)

	// Auto-inject rDNS PTR records for active node listeners so 'nslookup' displays Node ID
	if ctx.NodeID != "" {
		existingPTRs := make(map[string]bool)
		for _, r := range allRecords {
			if r.Type == "PTR" {
				existingPTRs[r.Value] = true
			}
		}

		for _, l := range ctx.ActiveListeners {
			if l.IP != "0.0.0.0" && l.IP != "::" && !existingPTRs[l.IP] {
				allRecords = append(allRecords, model.LocalRecord{
					Domain: ctx.NodeID,
					Type:   "PTR",
					Value:  l.IP,
					TTL:    3600,
				})
				allRecords = append(allRecords, model.LocalRecord{
					Domain: ctx.NodeID,
					Type:   "A",
					Value:  l.IP,
					TTL:    3600,
				})
			}
		}
	}
	ctx.LocalRecords = allRecords

	if len(ctx.LocalRecords) > 0 {
		recordsBuf := new(bytes.Buffer)
		if err := g.recordsTmpl.Execute(recordsBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering local-records.conf: %w", err)
		}
		recordsBytes := recordsBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/local-records.conf",
			Content:      recordsBytes,
			Checksum:     calculateSHA256(recordsBytes),
		})
	} else {
		emptyBytes := []byte("# No custom local records configured\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/local-records.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 4. Render conf.d/safesearch.conf
	if ctx.SafeSearch.Enabled {
		ssBuf := new(bytes.Buffer)
		if err := g.safesearchTmpl.Execute(ssBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering safesearch.conf: %w", err)
		}
		ssBytes := ssBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/safesearch.conf",
			Content:      ssBytes,
			Checksum:     calculateSHA256(ssBytes),
		})
	} else {
		emptyBytes := []byte("# SafeSearch redirection is DISABLED\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/safesearch.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 5. Render conf.d/parental-control.conf
	if ctx.ParentalControl.Enabled {
		pcBuf := new(bytes.Buffer)
		if err := g.parentalTmpl.Execute(pcBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering parental-control.conf: %w", err)
		}
		pcBytes := pcBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/parental-control.conf",
			Content:      pcBytes,
			Checksum:     calculateSHA256(pcBytes),
		})
	} else {
		emptyBytes := []byte("# Parental Control is DISABLED (Adult websites allowed)\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/parental-control.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 6. Render conf.d/gambling-filter.conf
	if ctx.IsGamblingBlockActive {
		gamblingBuf := new(bytes.Buffer)
		if err := g.gamblingTmpl.Execute(gamblingBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering gambling-filter.conf: %w", err)
		}
		gamblingBytes := gamblingBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/gambling-filter.conf",
			Content:      gamblingBytes,
			Checksum:     calculateSHA256(gamblingBytes),
		})
	} else {
		emptyBytes := []byte("# Judi Online / Gambling Filter is DISABLED (Gambling websites allowed)\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/gambling-filter.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 7. Render conf.d/adblock-filter.conf
	if ctx.IsAdBlockActive {
		adBuf := new(bytes.Buffer)
		if err := g.adTmpl.Execute(adBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering adblock-filter.conf: %w", err)
		}
		adBytes := adBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/adblock-filter.conf",
			Content:      adBytes,
			Checksum:     calculateSHA256(adBytes),
		})
	} else {
		emptyBytes := []byte("# AdBlock Filter is DISABLED (Ad networks allowed)\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/adblock-filter.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 8. Render conf.d/malware-filter.conf
	if ctx.IsMalwareBlockActive {
		malBuf := new(bytes.Buffer)
		if err := g.malwareTmpl.Execute(malBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering malware-filter.conf: %w", err)
		}
		malBytes := malBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/malware-filter.conf",
			Content:      malBytes,
			Checksum:     calculateSHA256(malBytes),
		})
	} else {
		emptyBytes := []byte("# Malware Filter is DISABLED (Security threat feed bypassed)\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/malware-filter.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 9. Render conf.d/custom-filter.conf
	if ctx.IsCustomBlockActive {
		custBuf := new(bytes.Buffer)
		if err := g.customTmpl.Execute(custBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering custom-filter.conf: %w", err)
		}
		custBytes := custBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/custom-filter.conf",
			Content:      custBytes,
			Checksum:     calculateSHA256(custBytes),
		})
	} else {
		emptyBytes := []byte("# Custom Block Filter is DISABLED\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/custom-filter.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	// 10. Render conf.d/whitelist.conf
	if len(ctx.WhitelistedDomains) > 0 {
		wlBuf := new(bytes.Buffer)
		if err := g.whitelistTmpl.Execute(wlBuf, ctx); err != nil {
			return nil, fmt.Errorf("failed rendering whitelist.conf: %w", err)
		}
		wlBytes := wlBuf.Bytes()
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/whitelist.conf",
			Content:      wlBytes,
			Checksum:     calculateSHA256(wlBytes),
		})
	} else {
		emptyBytes := []byte("# No custom whitelist configured\n")
		result.Files = append(result.Files, RenderedFile{
			RelativePath: "conf.d/whitelist.conf",
			Content:      emptyBytes,
			Checksum:     calculateSHA256(emptyBytes),
		})
	}

	return result, nil
}

func calculateSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
