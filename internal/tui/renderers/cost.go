package renderers

import (
	"fmt"
	"strings"
)

// Cost display thresholds matching Claude Code's visual cost warnings.
const (
	costWarningThreshold = 1.0 // $1.00 -- show in warning color
	costErrorThreshold   = 5.0 // $5.00 -- show in error color
)

// FormatCost formats a cost value in dollars with 2 decimal places.
// Returns "$0.42" format.
func FormatCost(dollars float64) string {
	return fmt.Sprintf("$%.2f", dollars)
}

// FormatTokens formats a token count with human-readable suffixes.
// Returns "45.2k" for thousands or "1.2M" for millions.
func FormatTokens(count int) string {
	if count >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	}
	if count >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(count)/1_000)
	}
	return fmt.Sprintf("%d", count)
}

// RenderCostSummary renders turn cost and session cost with visual indicators.
// Shows cost in warning color when > $1 and error color when > $5.
// Matches Claude Code's cost display behavior.
func RenderCostSummary(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	turnCost := msg.Metadata["turn_cost"]
	sessionCost := msg.Metadata["session_cost"]
	modelBreakdown := msg.Metadata["model_breakdown"]

	// Turn cost
	if turnCost != "" {
		sb.WriteString(dimStyle.Render("Turn: "))
		sb.WriteString(turnCost)
	}

	// Session cost with threshold coloring
	if sessionCost != "" {
		if sb.Len() > 0 {
			sb.WriteString(dimStyle.Render("  |  "))
		}
		sb.WriteString(dimStyle.Render("Session: "))

		// Parse session cost for threshold check
		var costVal float64
		fmt.Sscanf(sessionCost, "$%f", &costVal)

		if costVal >= costErrorThreshold {
			sb.WriteString(errorStyle.Render(sessionCost))
		} else if costVal >= costWarningThreshold {
			sb.WriteString(warningStyle.Render(sessionCost))
		} else {
			sb.WriteString(sessionCost)
		}
	}

	// Per-model breakdown if available
	if modelBreakdown != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(modelBreakdown)))
	}

	return sb.String()
}

// RenderTokenUsage renders input/output/cache token counts with a context
// usage bar. Format: "In: Xk Out: Yk Cache: Zk" in dim text, followed by
// a visual context usage progress bar.
func RenderTokenUsage(msg DisplayMessage, width int) string {
	var sb strings.Builder

	inputTokens := msg.Metadata["input_tokens"]
	outputTokens := msg.Metadata["output_tokens"]
	cacheReadTokens := msg.Metadata["cache_read_tokens"]
	cacheWriteTokens := msg.Metadata["cache_write_tokens"]
	contextPercent := msg.Metadata["context_percent"]

	// Token counts line: "In: Xk  Out: Yk  Cache: Zk"
	var parts []string
	if inputTokens != "" {
		parts = append(parts, fmt.Sprintf("In: %s", inputTokens))
	}
	if outputTokens != "" {
		parts = append(parts, fmt.Sprintf("Out: %s", outputTokens))
	}
	if cacheReadTokens != "" || cacheWriteTokens != "" {
		cacheStr := "Cache:"
		if cacheReadTokens != "" {
			cacheStr += fmt.Sprintf(" R:%s", cacheReadTokens)
		}
		if cacheWriteTokens != "" {
			cacheStr += fmt.Sprintf(" W:%s", cacheWriteTokens)
		}
		parts = append(parts, cacheStr)
	}
	if len(parts) > 0 {
		sb.WriteString(dimStyle.Render(strings.Join(parts, "  ")))
	}

	// Context usage bar: [###########-------] 78%
	if contextPercent != "" {
		var pct int
		fmt.Sscanf(contextPercent, "%d", &pct)
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}

		barWidth := 20
		if width > 0 && width < 40 {
			barWidth = 10
		}
		filled := barWidth * pct / 100
		empty := barWidth - filled

		bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", empty)

		if sb.Len() > 0 {
			sb.WriteString("\n")
		}

		// Color the bar based on usage level
		barStr := fmt.Sprintf("[%s] %d%%", bar, pct)
		if pct >= 90 {
			sb.WriteString(errorStyle.Render(barStr))
		} else if pct >= 75 {
			sb.WriteString(warningStyle.Render(barStr))
		} else {
			sb.WriteString(dimStyle.Render(barStr))
		}
	}

	return sb.String()
}
