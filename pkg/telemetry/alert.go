package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"netora-dns/pkg/storage"
)

// AlertSeverity defines the urgency of an operational alert.
type AlertSeverity string

const (
	SeverityCritical AlertSeverity = "CRITICAL"
	SeverityWarning  AlertSeverity = "WARNING"
	SeverityInfo     AlertSeverity = "INFO"
)

// AlertEvent represents a structured, actionable NOC alert.
type AlertEvent struct {
	ID          string        `json:"id"`
	Timestamp   time.Time     `json:"timestamp"`
	NodeID      string        `json:"node_id"`
	RuleName    string        `json:"rule_name"`
	Severity    AlertSeverity `json:"severity"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Value       float64       `json:"value"`
	Threshold   float64       `json:"threshold"`
	Resolved    bool          `json:"resolved"`
}

// AlertEngine evaluates operational metrics against thresholds and emits structured alerts.
type AlertEngine struct {
	mu            sync.RWMutex
	nodeID        string
	store         storage.Store
	metrics       *Metrics
	activeAlerts  map[string]*AlertEvent
	historyAlerts []*AlertEvent
}

// NewAlertEngine initializes the alert evaluation engine.
func NewAlertEngine(nodeID string, store storage.Store, metrics *Metrics) *AlertEngine {
	if metrics == nil {
		metrics = DefaultMetrics
	}
	return &AlertEngine{
		nodeID:        nodeID,
		store:         store,
		metrics:       metrics,
		activeAlerts:  make(map[string]*AlertEvent),
		historyAlerts: make([]*AlertEvent, 0),
	}
}

// Evaluate performs a health and threshold evaluation across active metrics.
func (e *AlertEngine) Evaluate(ctx context.Context, unboundHealthy bool) []*AlertEvent {
	e.mu.Lock()
	defer e.mu.Unlock()

	var newEvents []*AlertEvent

	// Rule 1: DNS Node Down
	if !unboundHealthy {
		evt := e.triggerAlert("DNS_NODE_DOWN", SeverityCritical, "DNS Node Offline / Port 53 Degraded",
			"Unbound process is not responding to DNS queries on UDP/TCP port 53.", 0, 1)
		if evt != nil {
			newEvents = append(newEvents, evt)
		}
	} else {
		e.resolveAlert("DNS_NODE_DOWN")
	}

	// Rule 2: Cache Hit Collapse
	totalQueries := e.metrics.CacheHitsTotal + e.metrics.CacheMissesTotal
	if totalQueries > 200 {
		hitRatio := float64(e.metrics.CacheHitsTotal) / float64(totalQueries)
		if hitRatio < 0.50 {
			evt := e.triggerAlert("CACHE_HIT_COLLAPSE", SeverityWarning, "Cache Hit Collapse",
				fmt.Sprintf("DNS cache hit ratio dropped to %.1f%% (threshold: 50%%).", hitRatio*100), hitRatio, 0.50)
			if evt != nil {
				newEvents = append(newEvents, evt)
			}
		} else {
			e.resolveAlert("CACHE_HIT_COLLAPSE")
		}
	}

	// Rule 3: Servfail Spike
	if totalQueries > 100 && e.metrics.ServfailTotal > 0 {
		failRatio := float64(e.metrics.ServfailTotal) / float64(totalQueries)
		if failRatio > 0.05 {
			evt := e.triggerAlert("SERVFAIL_SPIKE", SeverityWarning, "SERVFAIL Error Spike",
				fmt.Sprintf("SERVFAIL response rate reached %.2f%% (threshold: 5%%). Upstream or DNSSEC issue likely.", failRatio*100),
				failRatio, 0.05)
			if evt != nil {
				newEvents = append(newEvents, evt)
			}
		} else {
			e.resolveAlert("SERVFAIL_SPIKE")
		}
	}

	// Rule 4: Upstream Latency Degradation
	for upstream, latency := range e.metrics.UpstreamLatencyMS {
		ruleKey := fmt.Sprintf("UPSTREAM_HIGH_LATENCY_%s", upstream)
		if latency > 150.0 {
			evt := e.triggerAlert(ruleKey, SeverityWarning, fmt.Sprintf("Upstream High Latency: %s", upstream),
				fmt.Sprintf("RTT to upstream %s exceeded %.1fms (threshold: 150ms).", upstream, latency), latency, 150.0)
			if evt != nil {
				newEvents = append(newEvents, evt)
			}
		} else {
			e.resolveAlert(ruleKey)
		}
	}

	return newEvents
}

func (e *AlertEngine) triggerAlert(ruleName string, severity AlertSeverity, title, desc string, val, threshold float64) *AlertEvent {
	if _, exists := e.activeAlerts[ruleName]; exists {
		return nil // Already active
	}

	evt := &AlertEvent{
		ID:          fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Timestamp:   time.Now().UTC(),
		NodeID:      e.nodeID,
		RuleName:    ruleName,
		Severity:    severity,
		Title:       title,
		Description: desc,
		Value:       val,
		Threshold:   threshold,
		Resolved:    false,
	}

	e.activeAlerts[ruleName] = evt
	e.historyAlerts = append(e.historyAlerts, evt)

	// Emit structured JSON log (Loki compatible)
	logJSON, _ := json.Marshal(map[string]interface{}{
		"level":       string(severity),
		"event_type":  "ALERT_TRIGGERED",
		"alert_id":    evt.ID,
		"rule_name":   ruleName,
		"node_id":     e.nodeID,
		"title":       title,
		"description": desc,
		"timestamp":   evt.Timestamp.Format(time.RFC3339),
	})
	log.Printf("[LOKI-LOG] %s", string(logJSON))

	return evt
}

func (e *AlertEngine) resolveAlert(ruleName string) {
	if evt, exists := e.activeAlerts[ruleName]; exists {
		evt.Resolved = true
		delete(e.activeAlerts, ruleName)

		// Emit resolution log
		logJSON, _ := json.Marshal(map[string]interface{}{
			"level":      "INFO",
			"event_type": "ALERT_RESOLVED",
			"alert_id":   evt.ID,
			"rule_name":  ruleName,
			"node_id":    e.nodeID,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
		})
		log.Printf("[LOKI-LOG] %s", string(logJSON))
	}
}

// GetActiveAlerts returns all currently active alerts.
func (e *AlertEngine) GetActiveAlerts() []*AlertEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*AlertEvent
	for _, a := range e.activeAlerts {
		result = append(result, a)
	}
	return result
}
