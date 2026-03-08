package github

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"

	"github.com/infocus7/omni/pkg/utils"
	"github.com/infocus7/omni/pkg/widgets"
	"golang.org/x/sync/errgroup"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var tmpl = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// renderTemplate executes a named template and returns the HTML.
func renderTemplate(name string, data interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render %s: %w", name, err)
	}
	return template.HTML(buf.String()), nil
}

// ---------------------------------------------------------------------------
// RatioWidget — author-to-reviewer ratio
// ---------------------------------------------------------------------------

// RatioData is the template data for the ratio widget.
type RatioData struct {
	Filter       string
	Ratio        string
	ApprovalRate string
	AuthoredPct  float64
	ReviewedPct  float64
}

// RatioWidget shows the author-to-reviewer ratio.
type RatioWidget struct {
	client *Client
}

func NewRatioWidget(c *Client) *RatioWidget {
	return &RatioWidget{client: c}
}

func (w *RatioWidget) Definition() widgets.WidgetDef {
	return widgets.WidgetDef{
		ID:          "github-ratio",
		PluginID:    "github",
		Name:        "Ratio",
		Description: "Author-to-reviewer ratio",
		Sizes: []widgets.SizeOption{
			{Name: "wide", W: 2, H: 1},
			{Name: "large", W: 2, H: 2},
			{Name: "full", W: 3, H: 2},
		},
	}
}

func (w *RatioWidget) Render(ctx context.Context, filter string, sizeName string) (template.HTML, error) {
	if ctx == nil {
		return "", utils.NilContextError
	}

	since := sinceFromFilter(filter)
	sinceQ := SinceQualifier(since)

	g, ctx := errgroup.WithContext(ctx)

	var authored, reviewed, approved int

	g.Go(func() error {
		var err error
		authored, err = w.client.FetchCount(ctx, "is:pr author:@me"+sinceQ)
		return err
	})
	g.Go(func() error {
		var err error
		reviewed, err = w.client.FetchCount(ctx, "is:pr reviewed-by:@me"+sinceQ)
		return err
	})
	g.Go(func() error {
		var err error
		approved, err = w.client.FetchCount(ctx, "is:pr review:approved reviewed-by:@me"+sinceQ)
		return err
	})

	if err := g.Wait(); err != nil {
		return "", err
	}

	data := &RatioData{
		Filter:       filter,
		Ratio:        FormatRatio(authored, reviewed),
		ApprovalRate: FormatApprovalRate(approved, reviewed),
		AuthoredPct:  CalcPercent(authored, reviewed),
		ReviewedPct:  CalcPercent(reviewed, authored),
	}

	return renderTemplate("ratio_"+sizeName+".tmpl", data)
}

// ---------------------------------------------------------------------------
// AuthoredWidget — pull requests you authored
// ---------------------------------------------------------------------------

// AuthoredData is the template data for the authored widget.
type AuthoredData struct {
	Filter        string
	AuthoredCount int
	MergedCount   int
	MergeRate     string
}

// AuthoredWidget shows authored PR stats.
type AuthoredWidget struct {
	client *Client
}

func NewAuthoredWidget(c *Client) *AuthoredWidget {
	return &AuthoredWidget{client: c}
}

func (w *AuthoredWidget) Definition() widgets.WidgetDef {
	return widgets.WidgetDef{
		ID:          "github-authored",
		PluginID:    "github",
		Name:        "Authored",
		Description: "Pull requests you authored",
		Sizes: []widgets.SizeOption{
			{Name: "small", W: 1, H: 1},
			{Name: "wide", W: 2, H: 2},
		},
	}
}

func (w *AuthoredWidget) Render(ctx context.Context, filter string, sizeName string) (template.HTML, error) {
	if ctx == nil {
		return "", utils.NilContextError
	}

	since := sinceFromFilter(filter)
	sinceQ := SinceQualifier(since)

	g, ctx := errgroup.WithContext(ctx)

	var authored, merged int

	g.Go(func() error {
		var err error
		authored, err = w.client.FetchCount(ctx, "is:pr author:@me"+sinceQ)
		return err
	})

	// Only fetch merged count for "wide" size.
	if sizeName == "wide" {
		g.Go(func() error {
			var err error
			merged, err = w.client.FetchCount(ctx, "is:pr is:merged author:@me"+sinceQ)
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return "", err
	}

	data := &AuthoredData{
		Filter:        filter,
		AuthoredCount: authored,
		MergedCount:   merged,
		MergeRate:     FormatMergeRate(merged, authored),
	}

	return renderTemplate("authored_"+sizeName+".tmpl", data)
}

// ---------------------------------------------------------------------------
// ReviewedWidget — pull requests you reviewed
// ---------------------------------------------------------------------------

// ReviewedData is the template data for the reviewed widget.
type ReviewedData struct {
	Filter        string
	ReviewedCount int
	ApprovedCount int
	ApprovalRate  string
}

// ReviewedWidget shows reviewed PR stats.
type ReviewedWidget struct {
	client *Client
}

func NewReviewedWidget(c *Client) *ReviewedWidget {
	return &ReviewedWidget{client: c}
}

func (w *ReviewedWidget) Definition() widgets.WidgetDef {
	return widgets.WidgetDef{
		ID:          "github-reviewed",
		PluginID:    "github",
		Name:        "Reviewed",
		Description: "Pull requests you reviewed",
		Sizes: []widgets.SizeOption{
			{Name: "small", W: 1, H: 1},
			{Name: "wide", W: 2, H: 2},
		},
	}
}

func (w *ReviewedWidget) Render(ctx context.Context, filter string, sizeName string) (template.HTML, error) {
	if ctx == nil {
		return "", utils.NilContextError
	}

	since := sinceFromFilter(filter)
	sinceQ := SinceQualifier(since)

	g, ctx := errgroup.WithContext(ctx)

	var reviewed, approved int

	g.Go(func() error {
		var err error
		reviewed, err = w.client.FetchCount(ctx, "is:pr reviewed-by:@me"+sinceQ)
		return err
	})

	// Only fetch approved count for "wide" size.
	if sizeName == "wide" {
		g.Go(func() error {
			var err error
			approved, err = w.client.FetchCount(ctx, "is:pr review:approved reviewed-by:@me"+sinceQ)
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return "", err
	}

	data := &ReviewedData{
		Filter:        filter,
		ReviewedCount: reviewed,
		ApprovedCount: approved,
		ApprovalRate:  FormatApprovalRate(approved, reviewed),
	}

	return renderTemplate("reviewed_"+sizeName+".tmpl", data)
}

// ---------------------------------------------------------------------------
// RightNowWidget — open PRs, review debt, and assigned issues
// ---------------------------------------------------------------------------

// RightNowData is the template data for the right-now widget.
type RightNowData struct {
	OpenCount     int
	ReviewDebt    int
	AssignedCount int
}

// RightNowWidget shows live-state counts: open PRs, review debt, assigned issues.
type RightNowWidget struct {
	client *Client
}

func NewRightNowWidget(c *Client) *RightNowWidget {
	return &RightNowWidget{client: c}
}

func (w *RightNowWidget) Definition() widgets.WidgetDef {
	return widgets.WidgetDef{
		ID:          "github-rightnow",
		PluginID:    "github",
		Name:        "Right Now",
		Description: "Open PRs, review debt, and assigned issues",
		Sizes: []widgets.SizeOption{
			{Name: "tall", W: 1, H: 2},
			{Name: "wide", W: 3, H: 2},
		},
	}
}

func (w *RightNowWidget) Render(ctx context.Context, _ string, sizeName string) (template.HTML, error) {
	if ctx == nil {
		return "", utils.NilContextError
	}

	g, ctx := errgroup.WithContext(ctx)

	var open, debt, assigned int

	g.Go(func() error {
		var err error
		open, err = w.client.FetchCount(ctx, "is:pr is:open author:@me")
		return err
	})
	g.Go(func() error {
		var err error
		debt, err = w.client.FetchCount(ctx, "is:pr is:open review-requested:@me")
		return err
	})
	g.Go(func() error {
		var err error
		assigned, err = w.client.FetchCount(ctx, "is:issue is:open assignee:@me")
		return err
	})

	if err := g.Wait(); err != nil {
		return "", err
	}

	data := &RightNowData{
		OpenCount:     open,
		ReviewDebt:    debt,
		AssignedCount: assigned,
	}

	return renderTemplate("rightnow_"+sizeName+".tmpl", data)
}
