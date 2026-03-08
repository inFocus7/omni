package plugins

import (
	"context"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v83/github"
	ghplugin "github.com/infocus7/dash/pkg/plugins/github"
	"github.com/infocus7/dash/pkg/settings"
	"github.com/infocus7/dash/pkg/widgets"
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

// RenderedWidget is a pre-rendered widget ready for the dashboard template.
type RenderedWidget struct {
	ID       string
	PluginID string
	SizeName string
	W, H     int
	HTML     template.HTML
}

// DashboardData is passed to the dashboard template.
type DashboardData struct {
	ActiveFilter string
	Widgets      []RenderedWidget
	EditMode     bool
}

type PluginManager struct {
	ctx          context.Context
	githubClient *ghplugin.Client
	settings     *settings.Settings
	Registry     *widgets.Registry
}

func NewPluginManager(ctx context.Context, s *settings.Settings) (*PluginManager, error) {
	ghClient, err := ghplugin.NewClient()
	if err != nil {
		return nil, err
	}

	reg := widgets.NewRegistry()
	reg.Register(ghplugin.NewRatioWidget(ghClient))
	reg.Register(ghplugin.NewAuthoredWidget(ghClient))
	reg.Register(ghplugin.NewReviewedWidget(ghClient))
	reg.Register(ghplugin.NewRightNowWidget(ghClient))

	return &PluginManager{
		ctx:          ctx,
		githubClient: ghClient,
		settings:     s,
		Registry:     reg,
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

// findSizeOption searches for a size option by name in a widget definition.
// Returns the size option and true if found, otherwise returns zero value and false.
func findSizeOption(def widgets.WidgetDef, sizeName string) (widgets.SizeOption, bool) {
	for _, s := range def.Sizes {
		if s.Name == sizeName {
			return s, true
		}
	}
	return widgets.SizeOption{}, false
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

// FetchDashboardWidgets fetches and pre-renders pinned widgets from settings.
func (pm *PluginManager) FetchDashboardWidgets(filter string) ([]RenderedWidget, error) {
	pinned := pm.settings.Dashboard.Widgets
	if len(pinned) == 0 {
		return nil, nil
	}

	// Sort by position
	sorted := make([]settings.DashboardWidget, len(pinned))
	copy(sorted, pinned)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Position < sorted[j].Position
	})

	type result struct {
		index int
		rw    RenderedWidget
	}

	results := make([]result, len(sorted))
	g, ctx := errgroup.WithContext(pm.ctx)

	for i, dw := range sorted {
		i, dw := i, dw
		g.Go(func() error {
			w, ok := pm.Registry.Get(dw.ID)
			if !ok {
				return nil // skip unknown widgets
			}

			def := w.Definition()
			sizeName := dw.SizeName
			sizeOpt, found := findSizeOption(def, sizeName)
			if !found && len(def.Sizes) > 0 {
				sizeOpt = def.Sizes[0]
				sizeName = sizeOpt.Name
			}

			html, err := w.Render(ctx, filter, sizeName)
			if err != nil {
				// Render error widget instead of failing the whole dashboard
				errorHTML := renderWidgetError(dw.ID, err)
				results[i] = result{
					index: i,
					rw: RenderedWidget{
						ID:       dw.ID,
						PluginID: def.PluginID,
						SizeName: sizeName,
						W:        sizeOpt.W,
						H:        sizeOpt.H,
						HTML:     template.HTML(errorHTML),
					},
				}
				return nil
			}

			results[i] = result{
				index: i,
				rw: RenderedWidget{
					ID:       dw.ID,
					PluginID: def.PluginID,
					SizeName: sizeName,
					W:        sizeOpt.W,
					H:        sizeOpt.H,
					HTML:     html,
				},
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	rendered := make([]RenderedWidget, 0, len(results))
	for _, r := range results {
		if r.rw.ID != "" {
			rendered = append(rendered, r.rw)
		}
	}
	return rendered, nil
}

// renderWidgetError creates an HTML error state for a widget
func renderWidgetError(widgetID string, err error) string {
	escapedID := template.HTMLEscapeString(widgetID)
	escapedErr := template.HTMLEscapeString(err.Error())
	return fmt.Sprintf(`
		<div class="widget-error" data-widget-id="%s" data-error="%s">
			<div class="widget-error-icon">
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<circle cx="12" cy="12" r="10"/>
					<line x1="12" y1="8" x2="12" y2="12"/>
					<line x1="12" y1="16" x2="12.01" y2="16"/>
				</svg>
			</div>
			<div class="widget-error-title">Failed to load widget</div>
			<div class="widget-error-message">%s</div>
		</div>
	`, escapedID, escapedErr, escapedErr)
}

// RenderWidgetPreview renders a single widget at a given size for the preview API.
func (pm *PluginManager) RenderWidgetPreview(widgetID, sizeName, filter string) (template.HTML, int, int, error) {
	w, ok := pm.Registry.Get(widgetID)
	if !ok {
		return "", 0, 0, fmt.Errorf("unknown widget: %s", widgetID)
	}

	def := w.Definition()
	sizeOpt, found := findSizeOption(def, sizeName)
	if !found {
		return "", 0, 0, fmt.Errorf("unknown size %q for widget %s", sizeName, widgetID)
	}

	html, err := w.Render(pm.ctx, filter, sizeName)
	if err != nil {
		return "", 0, 0, err
	}

	return html, sizeOpt.W, sizeOpt.H, nil
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
