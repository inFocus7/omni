package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"os"

	"github.com/google/go-github/v83/github"
	"github.com/infocus7/dash/internal/cache"
	"github.com/infocus7/dash/pkg/utils"
	"golang.org/x/sync/errgroup"
)

type PRCodeStats struct {
	TotalAdditions int
	TotalDeletions int
}

type Client struct {
	client     *github.Client
	cache      *cache.SimpleCache[[]*github.Issue]
	countCache *cache.SimpleCache[int]
	statsCache *cache.SimpleCache[PRCodeStats]
}

func NewClient() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is not set (must have 'repo' scope to include private repositories)")
	}

	client := github.NewClient(nil).WithAuthToken(token)
	return &Client{
		client:     client,
		cache:      cache.NewSimpleCache[[]*github.Issue](cache.DefaultTTL),
		countCache: cache.NewSimpleCache[int](cache.DefaultTTL),
		statsCache: cache.NewSimpleCache[PRCodeStats](cache.DefaultTTL),
	}, nil
}

// SinceQualifier formats a time.Time into a GitHub search qualifier.
// Returns empty string for zero time (no date restriction).
func SinceQualifier(since time.Time) string {
	if since.IsZero() {
		return ""
	}
	return " created:>=" + since.Format("2006-01-02")
}

func (c *Client) FetchPullRequests(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr author:@me" + SinceQualifier(since)
	return c.searchIssuesParallel(ctx, "prs", query, opts)
}

func (c *Client) FetchReviews(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr reviewed-by:@me" + SinceQualifier(since)
	return c.searchIssuesParallel(ctx, "reviews", query, opts)
}

func (c *Client) FetchApprovals(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr review:approved reviewed-by:@me" + SinceQualifier(since)
	return c.searchIssuesParallel(ctx, "approvals", query, opts)
}

func (c *Client) FetchMergedPRs(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr is:merged author:@me" + SinceQualifier(since)
	return c.searchIssuesParallel(ctx, "merged", query, opts)
}

func (c *Client) FetchOpenPRs(ctx context.Context) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr is:open author:@me"
	return c.searchIssuesParallel(ctx, "open", query, opts)
}

// FetchReviewRequested returns PRs where review is currently requested from the authenticated user.
func (c *Client) FetchReviewRequested(ctx context.Context) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr is:open review-requested:@me"
	return c.searchIssuesParallel(ctx, "review-requested", query, opts)
}

// FetchPRCodeStats fetches additions/deletions for each PR via individual API calls.
// Results are cached under cacheKey.
func (c *Client) FetchPRCodeStats(ctx context.Context, prs []*github.Issue, cacheKey string) (PRCodeStats, error) {
	if cached, err := c.statsCache.Get(cacheKey); err == nil {
		return cached, nil
	}

	const maxConcurrent = 30
	sem := make(chan struct{}, maxConcurrent)

	var (
		mu         sync.Mutex
		totalAdded int
		totalDel   int
	)

	g, gctx := errgroup.WithContext(ctx)

	for _, pr := range prs {
		pr := pr
		if pr.RepositoryURL == nil || pr.Number == nil {
			continue
		}

		parts := strings.Split(*pr.RepositoryURL, "/")
		if len(parts) < 2 {
			continue
		}
		owner := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		number := *pr.Number

		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			result, _, err := c.client.PullRequests.Get(gctx, owner, repo, number)
			if err != nil {
				return err
			}

			mu.Lock()
			if result.Additions != nil {
				totalAdded += *result.Additions
			}
			if result.Deletions != nil {
				totalDel += *result.Deletions
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return PRCodeStats{}, err
	}

	stats := PRCodeStats{
		TotalAdditions: totalAdded,
		TotalDeletions: totalDel,
	}

	if err := c.statsCache.Set(cacheKey, stats); err != nil {
		return PRCodeStats{}, err
	}

	return stats, nil
}

// FetchCount returns the total result count for a query using a single lightweight API call.
// Only one result is fetched; the total_count field covers the full match set.
func (c *Client) FetchCount(ctx context.Context, query string) (int, error) {
	if ctx == nil {
		return 0, utils.NilContextError
	}

	key := "count:" + query
	if cached, err := c.countCache.Get(key); err == nil {
		return cached, nil
	}

	opts := &github.SearchOptions{ListOptions: github.ListOptions{PerPage: 1}}
	result, _, err := c.client.Search.Issues(ctx, query, opts)
	if err != nil {
		return 0, err
	}

	count := result.GetTotal()
	if err := c.countCache.Set(key, count); err != nil {
		return 0, err
	}
	return count, nil
}

// FetchTeamOpenPRs returns open PRs across watched orgs/repos.
// watched entries must be valid GitHub Search qualifiers (e.g. "org:myorg", "repo:owner/repo").
func (c *Client) FetchTeamOpenPRs(ctx context.Context, watched []string) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}
	if len(watched) == 0 {
		return nil, nil
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr is:open " + strings.Join(watched, " ")
	cacheKey := "team-prs:" + strings.Join(watched, ",")
	return c.searchIssuesParallel(ctx, cacheKey, query, opts)
}

// searchIssuesParallel fetches all pages concurrently after an initial page-1 fetch
// to discover LastPage. Safe for large result sets (up to GitHub's 1000-item cap).
func (c *Client) searchIssuesParallel(ctx context.Context, cachePrefix, query string, opts *github.SearchOptions) ([]*github.Issue, error) {
	key := createCacheKey(cachePrefix, query, opts)
	if cached, err := c.cache.Get(key); err == nil {
		return cached, nil
	}

	// Fetch page 1 to determine total page count.
	firstResult, firstResp, err := c.client.Search.Issues(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	lastPage := firstResp.LastPage
	if lastPage == 0 {
		// Single page — cache and return.
		issues := firstResult.Issues
		if err := c.cache.Set(key, issues); err != nil {
			return nil, err
		}
		return issues, nil
	}

	// Pre-allocate one slot per page; goroutines write to distinct indices (no mutex needed).
	allPages := make([][]*github.Issue, lastPage)
	allPages[0] = firstResult.Issues

	g, gctx := errgroup.WithContext(ctx)
	for pageNum := 2; pageNum <= lastPage; pageNum++ {
		pageNum := pageNum
		g.Go(func() error {
			pageOpts := &github.SearchOptions{
				ListOptions: github.ListOptions{PerPage: opts.PerPage, Page: pageNum},
			}
			result, _, err := c.client.Search.Issues(gctx, query, pageOpts)
			if err != nil {
				return err
			}
			allPages[pageNum-1] = result.Issues
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var allIssues []*github.Issue
	for _, page := range allPages {
		allIssues = append(allIssues, page...)
	}
	if err := c.cache.Set(key, allIssues); err != nil {
		return nil, err
	}
	return allIssues, nil
}

// FetchAssignedIssues returns open issues currently assigned to the authenticated user.
func (c *Client) FetchAssignedIssues(ctx context.Context) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:issue is:open assignee:@me"
	return c.searchIssuesParallel(ctx, "assigned", query, opts)
}

func createCacheKey(prefix string, query string, opts *github.SearchOptions) string {
	return fmt.Sprintf("%s:%s:%d", prefix, query, opts.PerPage)
}
