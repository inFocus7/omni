package github

import (
	"context"
	"errors"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/infocus7/dashie/pkg/utils"

	"os"
)

// Note: I used the GitHub repository for quick setup.
// I plan on moving to doing graphql queries with a custom client to get more fine-tuned control over the requests and responses.

// TODO: In-memory cache of results with a 30min TTL to avoid too many requests to the GitHub API?

// NOTE: Your GitHub PAT must have `repo` scope (not just `public_repo`) to include private repositories in search results.

type Client struct {
	client *github.Client
}

func NewClient() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is not set (must have 'repo' scope to include private repositories)")
	}

	client := github.NewClient(nil).WithAuthToken(token)
	return &Client{
		client: client,
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
