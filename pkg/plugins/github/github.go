package github

import (
	"context"
	"errors"

	"github.com/google/go-github/v83/github"
	"github.com/infocus7/dashie/pkg/utils"

	"os"
)

// Note: I used the GitHub repository for quick setup.
// I plan on moving to doing graphql queries with a custom client to get more fine-tuned control over the requests and responses.

// TODO: In-memory cache of results with a 30min TTL to avoid too many requests to the GitHub API?

type Client struct {
	ctx    context.Context
	client *github.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("github PAT token is not set through GITHUB_AUTH_TOKEN")
	}

	client := github.NewClient(nil).WithAuthToken(token)
	return &Client{
		ctx:    ctx,
		client: client,
	}, nil
}

func (c *Client) FetchPullRequests() ([]*github.Issue, error) {
	if c.ctx == nil {
		return nil, utils.NilContextError
	}

	var allIssues []*github.Issue
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		result, resp, err := c.client.Search.Issues(c.ctx, "is:pr author:@me", opts)
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

func (c *Client) FetchReviews() ([]*github.Issue, error) {
	if c.ctx == nil {
		return nil, utils.NilContextError
	}

	var allIssues []*github.Issue
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		result, resp, err := c.client.Search.Issues(c.ctx, "is:pr reviewed-by:@me", opts)
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

func (c *Client) FetchApprovals() ([]*github.Issue, error) {
	if c.ctx == nil {
		return nil, utils.NilContextError
	}

	var allIssues []*github.Issue
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		result, resp, err := c.client.Search.Issues(c.ctx, "is:pr review:approved reviewed-by:@me", opts)
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

func (c *Client) FetchFollowers() ([]*github.User, error) {
	if c.ctx == nil {
		return nil, utils.NilContextError
	}

	var allUsers []*github.User
	opts := &github.ListOptions{PerPage: 100}
	for {
		users, resp, err := c.client.Users.ListFollowers(c.ctx, "", opts)
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
