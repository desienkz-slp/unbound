package telemetry

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// UnboundStatsCollector periodically queries unbound-control stats_noreset
// and translates runtime metrics into Prometheus telemetry.
type UnboundStatsCollector struct {
	metrics  *Metrics
	interval time.Duration
	unboundBin string
}

// NewUnboundStatsCollector initializes a collector.
func NewUnboundStatsCollector(m *Metrics, interval time.Duration) *UnboundStatsCollector {
	if m == nil {
		m = DefaultMetrics
	}
	if interval < 1*time.Second {
		interval = 5 * time.Second
	}
	return &UnboundStatsCollector{
		metrics:    m,
		interval:   interval,
		unboundBin: "unbound-control",
	}
}

// Start runs the stats collection loop until the context is canceled.
func (c *UnboundStatsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Initial run
	c.CollectOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CollectOnce(ctx)
		}
	}
}

// CollectOnce executes one round of stats collection.
func (c *UnboundStatsCollector) CollectOnce(ctx context.Context) {
	cmd := exec.CommandContext(ctx, c.unboundBin, "stats_noreset")
	out, err := cmd.Output()
	if err != nil {
		// Log debug only to avoid noise if unbound is restarting
		return
	}

	stats := make(map[string]float64)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			vStr := strings.TrimSpace(parts[1])
			if v, err := strconv.ParseFloat(vStr, 64); err == nil {
				stats[k] = v
			}
		}
	}

	c.metrics.UpdateUnboundStats(stats)
}

// ParseStatsString parses raw unbound-control stats text directly.
func ParseStatsString(raw string) map[string]float64 {
	stats := make(map[string]float64)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			vStr := strings.TrimSpace(parts[1])
			if v, err := strconv.ParseFloat(vStr, 64); err == nil {
				stats[k] = v
			}
		}
	}
	return stats
}
