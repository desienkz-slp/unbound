package configengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
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
	ConfigDir            string
	GeneratedAt          string
	HasIPv6              bool
	HasTLSUpstream       bool
	ActiveListeners      []model.Listener
	ActiveClientNetworks   []model.ClientNetwork
	ActiveUpstreams        []model.UpstreamServer
	InsecureDomains        []string
	AdultBlockedDomains    []string
	GamblingBlockedDomains []string
	IsGamblingBlockActive  bool
	GamblingRedirectIP     string
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
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		domain := parts[0]
		if len(parts) >= 2 && (parts[0] == "0.0.0.0" || parts[0] == "127.0.0.1") {
			domain = parts[1]
		}
		domain = strings.TrimSuffix(domain, ".")
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

// Generator transforms DesiredState into deterministic Unbound configuration files.
type Generator struct {
	mainTmpl       *template.Template
	insecureTmpl   *template.Template
	recordsTmpl    *template.Template
	safesearchTmpl *template.Template
	parentalTmpl   *template.Template
	gamblingTmpl   *template.Template
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

	records, err := template.New("records").Parse(LocalRecordsTemplate)
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

	return &Generator{
		mainTmpl:       main,
		insecureTmpl:   insecure,
		recordsTmpl:    records,
		safesearchTmpl: safesearch,
		parentalTmpl:   parental,
		gamblingTmpl:   gambling,
	}, nil
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

	adultList := loadDomainList("/etc/unbound/blocklist/adult-domains.txt", DefaultAdultBlockedDomains)
	gamblingList := loadDomainList("/etc/unbound/blocklist/gambling-domains.txt", DefaultGamblingBlockedDomains)

	// Prepare sorted, deterministic collections
	ctx := &GeneratorContext{
		DesiredState:           state,
		ConfigDir:              configDir,
		GeneratedAt:            time.Now().UTC().Format(time.RFC3339),
		AdultBlockedDomains:    adultList,
		GamblingBlockedDomains: gamblingList,
		IsGamblingBlockActive:  isGamblingActive,
		GamblingRedirectIP:     gamblingRedirect,
	}

	// Filter and sort active listeners
	for _, l := range state.Listeners {
		if l.Enabled {
			if l.IsIPv6 || model.IsIPv6Address(l.IP) {
				ctx.HasIPv6 = true
			}
			ctx.ActiveListeners = append(ctx.ActiveListeners, l)
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

	// Filter and sort active upstreams
	for _, up := range state.Upstreams {
		if up.Enabled {
			ctx.ActiveUpstreams = append(ctx.ActiveUpstreams, up)
			if up.Type == model.UpstreamDoT || up.Type == model.UpstreamDoH || up.Port == 853 || up.TLSName != "" {
				ctx.HasTLSUpstream = true
			}
		}
	}
	sort.Slice(ctx.ActiveUpstreams, func(i, j int) bool {
		if ctx.ActiveUpstreams[i].IP != ctx.ActiveUpstreams[j].IP {
			return ctx.ActiveUpstreams[i].IP < ctx.ActiveUpstreams[j].IP
		}
		return ctx.ActiveUpstreams[i].Port < ctx.ActiveUpstreams[j].Port
	})

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
	// 2. Render conf.d/domain-insecure.conf
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

	return result, nil
}

func calculateSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
