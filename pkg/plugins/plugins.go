package plugins

import (
	"context"
	"time"

	"github.com/google/go-github/v83/github"
	ghplugin "github.com/infocus7/dashie/pkg/plugins/github"
	"golang.org/x/sync/errgroup"
)

type GitHubData struct {
	PullRequests []*github.Issue
	Reviews      []*github.Issue
	Approvals    []*github.Issue
	Followers    []*github.User
}

type DashboardData struct {
	ActiveFilter string
	GitHub       *GitHubData
}

type PluginManager struct {
	ctx          context.Context
	githubClient *ghplugin.Client
}

func NewPluginManager(ctx context.Context) (*PluginManager, error) {
	ghClient, err := ghplugin.NewClient()
	if err != nil {
		return nil, err
	}

	return &PluginManager{
		ctx:          ctx,
		githubClient: ghClient,
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
	case "1m":
		return now.AddDate(0, -1, 0)
	case "ytd":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	default: // "all"
		return time.Time{}
	}
}

func (pm *PluginManager) FetchDashboardData(filter string) (*DashboardData, error) {
	since := SinceFromFilter(filter)

	g, ctx := errgroup.WithContext(pm.ctx)

	var (
		prs       []*github.Issue
		reviews   []*github.Issue
		approvals []*github.Issue
		followers []*github.User
	)

	g.Go(func() error {
		var err error
		prs, err = pm.githubClient.FetchPullRequests(ctx, since)
		return err
	})
	g.Go(func() error {
		var err error
		reviews, err = pm.githubClient.FetchReviews(ctx, since)
		return err
	})
	g.Go(func() error {
		var err error
		approvals, err = pm.githubClient.FetchApprovals(ctx, since)
		return err
	})
	g.Go(func() error {
		var err error
		followers, err = pm.githubClient.FetchFollowers(ctx)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &DashboardData{
		ActiveFilter: filter,
		GitHub: &GitHubData{
			PullRequests: prs,
			Reviews:      reviews,
			Approvals:    approvals,
			Followers:    followers,
		},
	}, nil
}
