package configengine

import (
	"strings"
	"testing"

	"netora-dns/pkg/model"
)

func TestGenerator_RecursiveMode(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 4, 4096)
	state.ResolutionMode = model.ModeRecursive

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	if len(res.Files) == 0 {
		t.Fatalf("expected generated files, got 0")
	}

	mainConf := string(res.Files[0].Content)

	// Check core directives
	if !strings.Contains(mainConf, "module-config: \"validator iterator\"") {
		t.Errorf("expected DNSSEC validator iterator in recursive mode")
	}
	if !strings.Contains(mainConf, "auto-trust-anchor-file:") {
		t.Errorf("expected auto-trust-anchor-file")
	}
	if !strings.Contains(mainConf, "access-control: 0.0.0.0/0 refuse") {
		t.Errorf("expected default refuse ACL")
	}
	if strings.Contains(mainConf, "forward-zone:") {
		t.Errorf("did not expect forward-zone in pure recursive mode")
	}
}

func TestGenerator_ForwardingMode(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.ResolutionMode = model.ModeForwarding

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)
	if !strings.Contains(mainConf, "forward-zone:") {
		t.Errorf("expected forward-zone in forwarding mode")
	}
	if !strings.Contains(mainConf, "forward-addr: 1.1.1.1") {
		t.Errorf("expected 1.1.1.1 upstream")
	}
	if !strings.Contains(mainConf, "forward-addr: 8.8.8.8") {
		t.Errorf("expected 8.8.8.8 upstream")
	}
}

func TestGenerator_HybridMode(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.ResolutionMode = model.ModeHybrid

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)
	if !strings.Contains(mainConf, "forward-first: yes") {
		t.Errorf("expected forward-first: yes in hybrid mode")
	}
}

func TestGenerator_MultiListenerIPv4IPv6(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.Listeners = append(state.Listeners, model.Listener{
		ID:      "listen-ipv6",
		Name:    "Client IPv6",
		IP:      "2001:db8::1",
		Port:    53,
		Role:    model.RoleClient,
		IsIPv6:  true,
		UDP:     true,
		TCP:     true,
		Enabled: true,
	})

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	mainConf := string(res.Files[0].Content)
	if !strings.Contains(mainConf, "do-ip6: yes") {
		t.Errorf("expected do-ip6: yes when IPv6 listener is present")
	}
	if !strings.Contains(mainConf, "interface: 2001:db8::1") {
		t.Errorf("expected IPv6 interface binding")
	}
}

func TestGenerator_SecurityValidationOpenResolver(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	// Inject forbidden 0.0.0.0/0 allow
	state.ClientNetworks = append(state.ClientNetworks, model.ClientNetwork{
		ID:      "bad-open-resolver",
		CIDR:    "0.0.0.0/0",
		Action:  "allow",
		Enabled: true,
	})

	_, err = gen.Generate(state, "/etc/unbound")
	if err == nil {
		t.Fatalf("expected security error when configuring 0.0.0.0/0 allow, got nil")
	}
	if !strings.Contains(err.Error(), "open resolver") {
		t.Errorf("expected open resolver warning in error, got: %v", err)
	}
}

func TestGenerator_AllFilterConfigs(t *testing.T) {
	gen, err := NewGenerator()
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	state := model.NewDefaultDesiredState("node-01", 2, 2048)
	state.ParentalControl.Enabled = true
	state.GamblingBlock.Enabled = true
	state.AdBlock.Enabled = true
	state.MalwareBlock.Enabled = true
	state.CustomBlock.Enabled = true

	res, err := gen.Generate(state, "/etc/unbound")
	if err != nil {
		t.Fatalf("failed to generate config: %v", err)
	}

	expectedFiles := map[string]bool{
		"unbound.conf":                 false,
		"conf.d/domain-insecure.conf": false,
		"conf.d/local-records.conf":   false,
		"conf.d/safesearch.conf":      false,
		"conf.d/parental-control.conf":false,
		"conf.d/gambling-filter.conf":  false,
		"conf.d/adblock-filter.conf":   false,
		"conf.d/malware-filter.conf":   false,
		"conf.d/custom-filter.conf":    false,
		"conf.d/whitelist.conf":        false,
	}

	for _, f := range res.Files {
		if _, ok := expectedFiles[f.RelativePath]; ok {
			expectedFiles[f.RelativePath] = true
		}
	}

	for path, found := range expectedFiles {
		if !found {
			t.Errorf("expected generated file %s was missing", path)
		}
	}
}

