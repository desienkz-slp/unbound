package configengine

// MainUnboundConfTemplate is the production-grade Unbound configuration template.
const MainUnboundConfTemplate = `# ==============================================================================
# Netora DNS — Autonomous Managed Unbound Configuration
# Version ID: {{ .VersionID }}
# Node ID:    {{ .NodeID }}
# Generated:  {{ .GeneratedAt }}
# DO NOT EDIT DIRECTLY. Modifications will be overwritten by Netora Engine.
# ==============================================================================

server:
  # --- 1. Basic Server & Chroot ---
  chroot: ""
  directory: "{{ .ConfigDir }}"
  username: "unbound"
  do-ip4: yes
  do-ip6: {{ if .HasIPv6 }}yes{{ else }}no{{ end }}
  do-udp: yes
  do-tcp: yes

  # --- 2. Multi-IP & Interface Listeners ---
{{- range .ActiveListeners }}
  interface: {{ .IP }}{{ if ne .Port 53 }}@{{ .Port }}{{ end }}
{{- end }}

  # --- 3. Access Control Lists (Client Networks) ---
  # Default policy: Deny loopback-adjacent leaks and unconfigured subnets
  access-control: 0.0.0.0/0 refuse
  access-control: ::/0 refuse
{{- range .ActiveClientNetworks }}
  access-control: {{ .CIDR }} {{ .Action }}
{{- end }}

  # --- 4. Privacy & Hardening ---
  hide-identity: {{ if .Features.HideIdentity }}yes{{ else }}no{{ end }}
  hide-version: {{ if .Features.HideVersion }}yes{{ else }}no{{ end }}
  qname-minimisation: {{ if .Features.QnameMinimisation }}yes{{ else }}no{{ end }}
  harden-glue: {{ if .DNSSEC.HardenGlue }}yes{{ else }}no{{ end }}
  harden-dnssec-stripped: {{ if .DNSSEC.HardenDNSSECStripped }}yes{{ else }}no{{ end }}
  use-syslog: no
  logfile: "/var/log/unbound/unbound.log"
  log-queries: no
  log-replies: no
  verbosity: 0

  # --- 5. Cache & Resilience ---
  prefetch: {{ if .Features.Prefetch }}yes{{ else }}no{{ end }}
  prefetch-key: {{ if .Features.PrefetchKey }}yes{{ else }}no{{ end }}
  serve-expired: {{ if .Features.ServeExpired }}yes{{ else }}no{{ end }}
{{- if .Features.ServeExpired }}
  serve-expired-ttl: {{ .Features.ServeExpiredTTL }}
  serve-expired-client-timeout: {{ .Features.ServeExpiredClientTimeout }}
  serve-expired-reply-ttl: {{ .Features.ServeExpiredReplyTTL }}
{{- end }}
  cache-min-ttl: {{ .Cache.MinTTL }}
  cache-max-ttl: {{ .Cache.MaxTTL }}
  infra-host-ttl: {{ .Cache.InfraHostTTL }}

  # --- 6. Hardware Multi-Threading & Cache Allocations ---
  num-threads: {{ .Performance.NumThreads }}
  msg-cache-slabs: {{ .Performance.MsgCacheSlabs }}
  rrset-cache-slabs: {{ .Performance.RRSetCacheSlabs }}
  key-cache-slabs: {{ .Performance.KeyCacheSlabs }}
  infra-cache-slabs: {{ .Performance.InfraCacheSlabs }}

  msg-cache-size: {{ .Cache.MsgCacheSize }}
  rrset-cache-size: {{ .Cache.RRSetCacheSize }}
  key-cache-size: {{ .Cache.KeyCacheSize }}
  neg-cache-size: {{ .Cache.NegCacheSize }}

  # --- 7. Networking & High Throughput Buffers ---
  num-queries-per-thread: {{ .Performance.NumQueriesPerThread }}
  outgoing-range: {{ .Performance.OutgoingRange }}
  incoming-num-tcp: {{ .Performance.IncomingNumTCP }}
  so-rcvbuf: {{ .Performance.SoRcvBuf }}
  so-sndbuf: {{ .Performance.SoSndBuf }}
  so-reuseport: {{ if .Performance.SoReuseport }}yes{{ else }}no{{ end }}

  # --- 8. Telemetry & Statistics ---
  statistics-interval: 0
  extended-statistics: yes

  # --- 9. DNSSEC Configuration ---
{{- if .DNSSEC.Enabled }}
{{- if .DNSSEC.AutoTrustAnchorFile }}
  auto-trust-anchor-file: "{{ .DNSSEC.AutoTrustAnchorFile }}"
{{- end }}
{{- if .DNSSEC.RootHintsFile }}
  root-hints: "{{ .DNSSEC.RootHintsFile }}"
{{- end }}
  val-clean-additional: yes
  val-permissive-mode: {{ if .DNSSEC.ValPermissiveMode }}yes{{ else }}no{{ end }}
  module-config: "validator iterator"
{{- else }}
  module-config: "iterator"
{{- end }}
{{- if .HasTLSUpstream }}
  tls-cert-bundle: "/etc/ssl/certs/ca-certificates.crt"
{{- end }}

  # --- 10. Auxiliary Configuration Directives ---
  include: "{{ .ConfigDir }}/conf.d/*.conf"

# --- 11. Protected Remote Control ---
remote-control:
  control-enable: {{ if .RemoteControl.Enabled }}yes{{ else }}no{{ end }}
  control-port: {{ .RemoteControl.Port }}
  control-interface: {{ .RemoteControl.Interface }}
  control-use-cert: {{ if .RemoteControl.ControlUseCert }}yes{{ else }}no{{ end }}

# --- 12. Upstream & Resolution Architecture ---
{{- if eq .ResolutionMode "FORWARDING" }}
forward-zone:
  name: "."
{{- if .HasTLSUpstream }}
  forward-tls-upstream: yes
{{- end }}
{{- range .ActiveUpstreams }}
  forward-addr: {{ .IP }}{{ if ne .Port 53 }}@{{ .Port }}{{ end }}{{ if .TLSName }}#{{ .TLSName }}{{ end }}
{{- end }}
{{- else if eq .ResolutionMode "HYBRID" }}
forward-zone:
  name: "."
  forward-first: yes
{{- if .HasTLSUpstream }}
  forward-tls-upstream: yes
{{- end }}
{{- range .ActiveUpstreams }}
  forward-addr: {{ .IP }}{{ if ne .Port 53 }}@{{ .Port }}{{ end }}{{ if .TLSName }}#{{ .TLSName }}{{ end }}
{{- end }}
{{- end }}
`

// DomainInsecureTemplate renders explicit domain-insecure exceptions.
const DomainInsecureTemplate = `# ==============================================================================
# Netora DNS — Audited DNSSEC Insecure Exceptions
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- range .InsecureDomains }}
  domain-insecure: "{{ . }}"
{{- end }}
`

// LocalRecordsTemplate renders custom authoritative local records.
const LocalRecordsTemplate = `# ==============================================================================
# Netora DNS — Local Authoritative Records
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- range .LocalRecords }}
{{- if eq .Type "PTR" }}
  local-data-ptr: "{{ .Value }} {{ .Domain }}"
{{- else }}
  local-data: "{{ .Domain }} {{ if .TTL }}{{ .TTL }} {{ end }}{{ .Type }} {{ .Value }}"
{{- end }}
{{- end }}
`

// SafeSearchTemplate renders SafeSearch VIP overrides.
const SafeSearchTemplate = `# ==============================================================================
# Netora DNS — SafeSearch Engine Redirection
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- if .SafeSearch.GoogleRedirectIPv4 }}
  local-zone: "www.google.com." redirect
  local-data: "www.google.com. A {{ .SafeSearch.GoogleRedirectIPv4 }}"
  local-zone: "forcesafesearch.google.com." redirect
  local-data: "forcesafesearch.google.com. A {{ .SafeSearch.GoogleRedirectIPv4 }}"
{{- end }}
{{- if .SafeSearch.BingRedirectIPv4 }}
  local-zone: "www.bing.com." redirect
  local-data: "www.bing.com. A {{ .SafeSearch.BingRedirectIPv4 }}"
  local-zone: "strict.bing.com." redirect
  local-data: "strict.bing.com. A {{ .SafeSearch.BingRedirectIPv4 }}"
{{- end }}
`

// ParentalControlTemplate renders parental control adult domain blocking rules.
const ParentalControlTemplate = `# ==============================================================================
# Netora DNS — Parental Control & Adult Content Filter
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- if .ParentalControl.Enabled }}
{{- range .AdultBlockedDomains }}
{{- if $.ParentalControl.RedirectIP }}
  local-data: "{{ . }} A {{ $.ParentalControl.RedirectIP }}"
  local-data: "www.{{ . }} A {{ $.ParentalControl.RedirectIP }}"
{{- else }}
  local-zone: "{{ . }}." static
{{- end }}
{{- end }}
{{- else }}
  # Parental Control is DISABLED (Adult websites allowed)
{{- end }}
`

// GamblingFilterTemplate renders online gambling (judi online) domain blocking rules.
const GamblingFilterTemplate = `# ==============================================================================
# Netora DNS — Judi Online & Gambling Content Filter
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- if .IsGamblingBlockActive }}
{{- range .GamblingBlockedDomains }}
{{- if $.GamblingRedirectIP }}
  local-data: "{{ . }} A {{ $.GamblingRedirectIP }}"
  local-data: "www.{{ . }} A {{ $.GamblingRedirectIP }}"
{{- else }}
  local-zone: "{{ . }}." static
{{- end }}
{{- end }}
{{- else }}
  # Judi Online / Gambling Filter is DISABLED (Gambling websites allowed)
{{- end }}
`

// AdBlockTemplate renders ad, tracker, and popup blocking rules.
const AdBlockTemplate = `# ==============================================================================
# Netora DNS — Ads & Telemetry Filter
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- if .IsAdBlockActive }}
{{- range .AdsBlockedDomains }}
{{- if $.AdRedirectIP }}
  local-data: "{{ . }} A {{ $.AdRedirectIP }}"
  local-data: "www.{{ . }} A {{ $.AdRedirectIP }}"
{{- else }}
  local-zone: "{{ . }}." static
{{- end }}
{{- end }}
{{- else }}
  # AdBlock Filter is DISABLED (Ad networks allowed)
{{- end }}
`

// MalwareBlockTemplate renders malware, phishing and C2 botnet blocking rules.
const MalwareBlockTemplate = `# ==============================================================================
# Netora DNS — Malware, Phishing & Threat Feed
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- if .IsMalwareBlockActive }}
{{- range .MalwareBlockedDomains }}
{{- if $.MalwareRedirectIP }}
  local-data: "{{ . }} A {{ $.MalwareRedirectIP }}"
  local-data: "www.{{ . }} A {{ $.MalwareRedirectIP }}"
{{- else }}
  local-zone: "{{ . }}." static
{{- end }}
{{- end }}
{{- else }}
  # Malware Filter is DISABLED (Security threat feed bypassed)
{{- end }}
`

// CustomBlockTemplate renders custom ISP-specific blacklisted domains.
const CustomBlockTemplate = `# ==============================================================================
# Netora DNS — Custom ISP Blacklist Filter
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- if .IsCustomBlockActive }}
{{- range .CustomBlockedDomains }}
{{- if $.CustomRedirectIP }}
  local-data: "{{ . }} A {{ $.CustomRedirectIP }}"
  local-data: "www.{{ . }} A {{ $.CustomRedirectIP }}"
{{- else }}
  local-zone: "{{ . }}." static
{{- end }}
{{- end }}
{{- else }}
  # Custom Block Filter is DISABLED
{{- end }}
`

// WhitelistTemplate renders transparent passthrough exceptions for whitelisted domains.
const WhitelistTemplate = `# ==============================================================================
# Netora DNS — Whitelist & Transparent Exceptions
# Generated: {{ .GeneratedAt }}
# ==============================================================================

server:
{{- range .WhitelistedDomains }}
  local-zone: "{{ . }}." transparent
{{- end }}
`


