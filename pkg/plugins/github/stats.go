package github

import (
	"fmt"
	"time"
)

// sinceFromFilter converts a filter string into a time.Time representing the start of the window.
// Returns zero time for "all" (no date restriction).
func sinceFromFilter(filter string) time.Time {
	now := time.Now()
	switch filter {
	case "1d":
		return now.AddDate(0, 0, -1)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "1mo":
		return now.AddDate(0, -1, 0)
	case "ytd":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Time{}
	}
}

// FormatRatio formats the author-to-reviewer ratio.
// Returns "—" if both are zero, "0 : ∞" if authored is zero, otherwise "1 : X.X".
func FormatRatio(authored, reviewed int) string {
	switch {
	case authored == 0 && reviewed == 0:
		return "—"
	case authored == 0:
		return "0 : ∞"
	default:
		return fmt.Sprintf("1 : %.1f", float64(reviewed)/float64(authored))
	}
}

// CalcPercent calculates the percentage of numerator over total (numerator + denominator).
// Returns 50.0 if total is zero (default split).
func CalcPercent(numerator, denominator int) float64 {
	total := numerator + denominator
	if total == 0 {
		return 50.0
	}
	return float64(numerator) / float64(total) * 100
}

// FormatApprovalRate formats the approval rate as a percentage.
// Returns "—" if reviewed is zero, otherwise "XX%".
func FormatApprovalRate(approved, reviewed int) string {
	if reviewed > 0 {
		return fmt.Sprintf("%.0f%%", float64(approved)/float64(reviewed)*100)
	}
	return "—"
}

// FormatMergeRate formats the merge rate as a percentage.
// Returns "—" if authored is zero, otherwise "XX%".
func FormatMergeRate(merged, authored int) string {
	if authored > 0 {
		return fmt.Sprintf("%.0f%%", float64(merged)/float64(authored)*100)
	}
	return "—"
}
