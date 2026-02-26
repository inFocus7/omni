package github

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/infocus7/dashie/internal/cache"
	"github.com/infocus7/dashie/pkg/utils"

	"os"
)

// Note: I used the GitHub repository for quick setup.
// I plan on moving to doing graphql queries with a custom client to get more fine-tuned control over the requests and responses.

// TODO: In-memory cache of results with a 30min TTL to avoid too many requests to the GitHub API?

type Client struct {
	client *github.Client
	cache  *cache.SimpleCache[[]*github.Issue]
}

func NewClient() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is not set (must have 'repo' scope to include private repositories)")
	}

	client := github.NewClient(nil).WithAuthToken(token)
	return &Client{
		client: client,
		cache:  cache.NewSimpleCache[[]*github.Issue](cache.DefaultTTL),
	}, nil
}

func sinceQualifier(since time.Time) string {
	if since.IsZero() {
		return ""
	}
	return " created:>=" + since.Format("2006-01-02")
}

func (c *Client) FetchPullRequests(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	var allIssues []*github.Issue
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr author:@me" + sinceQualifier(since)

	key := createCacheKey("prs", query, opts)

	if cached, err := c.cache.Get(key); err == nil {
		fmt.Println("cache hit")
		return cached, nil
	}
	fmt.Println("cache miss")

	for {
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if err := c.cache.Set(key, allIssues); err != nil {
		fmt.Println("error setting cache:", err) // no need to panic
	}

	return allIssues, nil
}

func (c *Client) FetchReviews(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	var allIssues []*github.Issue
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr reviewed-by:@me" + sinceQualifier(since)

	key := createCacheKey("reviews", query, opts)

	if cached, err := c.cache.Get(key); err == nil {
		fmt.Println("cache hit")
		return cached, nil
	}
	fmt.Println("cache miss")

	for {
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if err := c.cache.Set(key, allIssues); err != nil {
		fmt.Println("error setting cache:", err)
	}
	return allIssues, nil
}

func (c *Client) FetchApprovals(ctx context.Context, since time.Time) ([]*github.Issue, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	var allIssues []*github.Issue
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	query := "is:pr review:approved reviewed-by:@me" + sinceQualifier(since)

	key := createCacheKey("approvals", query, opts)

	if cached, err := c.cache.Get(key); err == nil {
		fmt.Println("cache hit")
		return cached, nil
	}
	fmt.Println("cache miss")

	for {
		result, resp, err := c.client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, result.Issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if err := c.cache.Set(key, allIssues); err != nil {
		fmt.Println("error setting cache:", err)
	}
	return allIssues, nil
}

func (c *Client) FetchFollowers(ctx context.Context) ([]*github.User, error) {
	if ctx == nil {
		return nil, utils.NilContextError
	}

	var allUsers []*github.User
	opts := &github.ListOptions{PerPage: 100}
	for {
		users, resp, err := c.client.Users.ListFollowers(ctx, "", opts)
		if err != nil {
			return nil, err
		}
		allUsers = append(allUsers, users...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allUsers, nil
}

func createCacheKey(prefix string, query string, opts *github.SearchOptions) string {
	return fmt.Sprintf("%s:%s:%d", prefix, query, opts.PerPage)
}
