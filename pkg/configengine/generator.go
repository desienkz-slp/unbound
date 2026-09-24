package configengine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	ActiveListeners      []model.Listener
	ActiveClientNetworks []model.ClientNetwork
	ActiveUpstreams      []model.UpstreamServer
	InsecureDomains      []string
}

// Generator transforms DesiredState into deterministic Unbound configuration files.
type Generator struct {
	mainTmpl     *template.Template
	insecureTmpl *template.Template
	recordsTmpl  *template.Template
	safesearchTmpl *template.Template
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

	return &Generator{
		mainTmpl:       main,
		insecureTmpl:   insecure,
		recordsTmpl:    records,
		safesearchTmpl: safesearch,
	}, nil
}

// Generate renders the full set of configuration files from the desired state.
func (g *Generator) Generate(state *model.DesiredState, configDir string) (*RenderResult, error) {
	if errs := model.ValidateDesiredState(state); len(errs) > 0 {
		return nil, fmt.Errorf("cannot generate configuration: %v", errs[0])
	}

	// Prepare sorted, deterministic collections
	ctx := &GeneratorContext{
		DesiredState: state,
		ConfigDir:    configDir,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
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
	}

	// 3. Render conf.d/local-records.conf (if any)
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
	}

	// 4. Render conf.d/safesearch.conf (if enabled)
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
	}

	return result, nil
}

func calculateSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
