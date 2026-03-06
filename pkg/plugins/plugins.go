package plugins

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v83/github"
	ghplugin "github.com/infocus7/dashie/pkg/plugins/github"
	"github.com/infocus7/dashie/pkg/settings"
	"golang.org/x/sync/errgroup"
)

// GitHubCounts holds count-only stats for the dashboard — fetched with minimal API calls.
type GitHubCounts struct {
	Filter        string
	AuthoredCount int
	ReviewedCount int
	ApprovedCount int
	MergedCount   int
	OpenCount     int
	ReviewDebt    int
	AssignedCount int

	// Pre-computed display strings
	Ratio        string
	ApprovalRate string
	AuthoredPct  float64
	ReviewedPct  float64
	MergeRate    string
}

// GitHubDetailData holds full list data for the /github plugin page.
type GitHubDetailData struct {
	ActiveFilter    string
	OpenPRs         []*github.Issue
	ReviewRequested []*github.Issue
	AssignedIssues  []*github.Issue
	TeamPRs         []*github.Issue
	WatchedEntries  []string
	TotalAdded      string
	TotalRemoved    string
}

type DashboardData struct {
	ActiveFilter string
	GitHub       *GitHubCounts
}

type PluginManager struct {
	ctx          context.Context
	githubClient *ghplugin.Client
	settings     *settings.Settings
}

func NewPluginManager(ctx context.Context, s *settings.Settings) (*PluginManager, error) {
	ghClient, err := ghplugin.NewClient()
	if err != nil {
		return nil, err
	}

	return &PluginManager{
		ctx:          ctx,
		githubClient: ghClient,
		settings:     s,
	}, nil
}

// SinceFromFilter converts a filter string into a time.Time representing the start of the window.
// Returns zero time for "all" (no date restriction).
func SinceFromFilter(filter string) time.Time {
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
	default: // "all"
		return time.Time{}
	}
}

func sinceQualifier(since time.Time) string {
	if since.IsZero() {
		return ""
	}
	return " created:>=" + since.Format("2006-01-02")
}

// formatInt formats an integer with comma separators (e.g. 4231 -> "4,231").
func formatInt(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	offset := len(s) % 3
	b.WriteString(s[:offset])
	for i := offset; i < len(s); i += 3 {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// FetchDashboardData fetches count-only stats for the main dashboard.
func (pm *PluginManager) FetchDashboardData(filter string) (*DashboardData, error) {
	since := SinceFromFilter(filter)
	sinceQ := sinceQualifier(since)

	g, ctx := errgroup.WithContext(pm.ctx)

	var (
		authored int
		reviewed int
		approved int
		merged   int
		open     int
		debt     int
		assigned int
	)

	g.Go(func() error {
		var err error
		authored, err = pm.githubClient.FetchCount(ctx, "is:pr author:@me"+sinceQ)
		return err
	})
	g.Go(func() error {
		var err error
		reviewed, err = pm.githubClient.FetchCount(ctx, "is:pr reviewed-by:@me"+sinceQ)
		return err
	})
	g.Go(func() error {
		var err error
		approved, err = pm.githubClient.FetchCount(ctx, "is:pr review:approved reviewed-by:@me"+sinceQ)
		return err
	})
	g.Go(func() error {
		var err error
		merged, err = pm.githubClient.FetchCount(ctx, "is:pr is:merged author:@me"+sinceQ)
		return err
	})
	g.Go(func() error {
		var err error
		// No since qualifier — open is live state, not a windowed event.
		open, err = pm.githubClient.FetchCount(ctx, "is:pr is:open author:@me")
		return err
	})
	g.Go(func() error {
		var err error
		// No since qualifier — review requests are live state. is:open excludes merged/closed PRs.
		debt, err = pm.githubClient.FetchCount(ctx, "is:pr is:open review-requested:@me")
		return err
	})
	g.Go(func() error {
		var err error
		// No since qualifier — assignments are live state.
		assigned, err = pm.githubClient.FetchCount(ctx, "is:issue is:open assignee:@me")
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	var ratio string
	switch {
	case authored == 0 && reviewed == 0:
		ratio = "—"
	case authored == 0:
		ratio = "0 : ∞"
	default:
		ratio = fmt.Sprintf("1 : %.1f", float64(reviewed)/float64(authored))
	}

	var approvalRate string
	if reviewed > 0 {
		approvalRate = fmt.Sprintf("%.0f%%", float64(approved)/float64(reviewed)*100)
	} else {
		approvalRate = "—"
	}

	total := authored + reviewed
	authoredPct, reviewedPct := 50.0, 50.0
	if total > 0 {
		authoredPct = float64(authored) / float64(total) * 100
		reviewedPct = float64(reviewed) / float64(total) * 100
	}

	var mergeRate string
	if authored > 0 {
		mergeRate = fmt.Sprintf("%.0f%%", float64(merged)/float64(authored)*100)
	} else {
		mergeRate = "—"
	}

	return &DashboardData{
		ActiveFilter: filter,
		GitHub: &GitHubCounts{
			Filter:        filter,
			AuthoredCount: authored,
			ReviewedCount: reviewed,
			ApprovedCount: approved,
			MergedCount:   merged,
			OpenCount:     open,
			ReviewDebt:    debt,
			AssignedCount: assigned,
			Ratio:         ratio,
			ApprovalRate:  approvalRate,
			AuthoredPct:   authoredPct,
			ReviewedPct:   reviewedPct,
			MergeRate:     mergeRate,
		},
	}, nil
}

// FetchGitHubDetail fetches full list data for the /github plugin page.
// The filter only applies to prs (for code stats); the live-state lists ignore it.
func (pm *PluginManager) FetchGitHubDetail(filter string) (*GitHubDetailData, error) {
	since := SinceFromFilter(filter)

	g, ctx := errgroup.WithContext(pm.ctx)

	watched := pm.settings.GitHub.Watched

	var (
		prs             []*github.Issue
		openPRs         []*github.Issue
		reviewRequested []*github.Issue
		assignedIssues  []*github.Issue
		teamPRs         []*github.Issue
	)

	g.Go(func() error {
		var err error
		prs, err = pm.githubClient.FetchPullRequests(ctx, since)
		return err
	})
	g.Go(func() error {
		var err error
		openPRs, err = pm.githubClient.FetchOpenPRs(ctx)
		return err
	})
	g.Go(func() error {
		var err error
		reviewRequested, err = pm.githubClient.FetchReviewRequested(ctx)
		return err
	})
	g.Go(func() error {
		var err error
		assignedIssues, err = pm.githubClient.FetchAssignedIssues(ctx)
		return err
	})
	g.Go(func() error {
		var err error
		teamPRs, err = pm.githubClient.FetchTeamOpenPRs(ctx, watched)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Fetch per-PR code stats (depends on prs being ready).
	stats, err := pm.githubClient.FetchPRCodeStats(pm.ctx, prs, "prstats:"+filter)
	if err != nil {
		return nil, err
	}

	return &GitHubDetailData{
		ActiveFilter:    filter,
		OpenPRs:         openPRs,
		ReviewRequested: reviewRequested,
		AssignedIssues:  assignedIssues,
		TeamPRs:         teamPRs,
		WatchedEntries:  watched,
		TotalAdded:      "+" + formatInt(stats.TotalAdditions),
		TotalRemoved:    "-" + formatInt(stats.TotalDeletions),
	}, nil
}
