package repl

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	repltheme "github.com/mochow13/keen-agent/internal/cli/repl/theme"
	"github.com/mochow13/keen-agent/internal/llm"
)

const (
	compactionSuggestThreshold = 70.0
)

type contextStatus struct {
	CurrentTokens     int
	ContextWindow     int
	Percent           float64
	KnownWindow       bool
	KnownTokens       bool
	TotalInputTokens  int
	TotalOutputTokens int
}

func (s *contextStatus) AddUsage(usage *llm.TokenUsage) {
	if usage == nil {
		return
	}
	s.TotalInputTokens += usage.InputTokens
	s.TotalOutputTokens += usage.OutputTokens
}

func (s *contextStatus) ResetTotals() {
	s.TotalInputTokens = 0
	s.TotalOutputTokens = 0
}

func (s contextStatus) ShouldSuggestCompaction() bool {
	return s.KnownWindow && s.KnownTokens && s.Percent >= compactionSuggestThreshold
}

func usagePercent(currentTokens, contextWindow int) float64 {
	if currentTokens <= 0 || contextWindow <= 0 {
		return 0
	}
	percent := (float64(currentTokens) * 100.0) / float64(contextWindow)
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}

func (m replModel) computeContextStatus() contextStatus {
	var providerID, modelID string
	if m.ctx != nil && m.ctx.cfg != nil {
		providerID = m.ctx.cfg.Provider
		modelID = m.ctx.cfg.Model
	}

	var contextWindow int
	var knownWindow bool
	if m.ctx != nil && m.ctx.registry != nil && providerID != "" && modelID != "" {
		contextWindow, knownWindow = m.ctx.registry.GetModelContextWindow(providerID, modelID)
	}

	var currentTokens int
	var knownTokens bool
	if m.appState != nil {
		if usage := m.appState.GetLastUsage(); usage != nil {
			currentTokens = usage.InputTokens
			knownTokens = true
		}
	}

	status := contextStatus{
		CurrentTokens:     currentTokens,
		ContextWindow:     contextWindow,
		KnownWindow:       knownWindow,
		KnownTokens:       knownTokens,
		TotalInputTokens:  m.contextStatus.TotalInputTokens,
		TotalOutputTokens: m.contextStatus.TotalOutputTokens,
	}
	if knownWindow && knownTokens {
		status.Percent = usagePercent(currentTokens, contextWindow)
	}
	return status
}

func (m *replModel) refreshContextStatus() {
	if m == nil {
		return
	}
	m.contextStatus = m.computeContextStatus()
}

func formatPercent(percent float64) string {
	p := strconv.FormatFloat(percent, 'f', 2, 64)
	p = strings.TrimRight(p, "0")
	p = strings.TrimRight(p, ".")
	return p + "%"
}

func contextPercentStyle(percent float64) lipgloss.Style {
	if percent >= 95 {
		return repltheme.ContextStatusPercentCriticalStyle
	}
	if percent >= 80 {
		return repltheme.ContextStatusPercentWarnStyle
	}
	return repltheme.ContextStatusPercentStyle
}

func formatCompactTokens(n int) string {
	if n < 1000 {
		return strconv.Itoa(n)
	}
	if n < 1_000_000 {
		v := float64(n) / 1000.0
		if v >= 999.95 {
			return formatCompactFloat(v/1000.0) + "M"
		}
		return formatCompactFloat(v) + "k"
	}
	return formatCompactFloat(float64(n)/1_000_000.0) + "M"
}

func formatCompactFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 1, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func renderContextStatus(status contextStatus) string {
	if !status.KnownWindow || status.ContextWindow <= 0 {
		return repltheme.ContextStatusUnknownStyle.Render("N/A")
	}
	if !status.KnownTokens {
		return repltheme.MetaLabelStyle.Render("context:") + " " + repltheme.ContextStatusPercentStyle.Render("0.0%") + " • " + repltheme.MetaLabelStyle.Render("0 ↑ / 0 ↓")
	}

	percent := contextPercentStyle(status.Percent).Render(formatPercent(status.Percent))
	result := repltheme.MetaLabelStyle.Render("context:") + " " + percent

	if status.TotalInputTokens > 0 || status.TotalOutputTokens > 0 {
		tokensText := formatCompactTokens(status.TotalInputTokens) + " ↑ / " + formatCompactTokens(status.TotalOutputTokens) + " ↓"
		result += " • " + repltheme.MetaLabelStyle.Render(tokensText)
	}

	return result
}
