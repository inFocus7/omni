package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-github/v83/github"

	"os"
)

// Note: I used the GitHub repository for quick setup.
// I plan on moving to doing graphql queries with a custom client to get more fine-tuned control over the requests and responses.

// TODO: In-memory cache of results with a 30min TTL to avoid too many requests to the GitHub API?

type Client struct {
	client *github.Client
}

func NewGithubClient(ctx context.Context) (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("github PAT token is not set through GITHUB_AUTH_TOKEN")
	}

	client := github.NewClient(nil).WithAuthToken(token)
	return &Client{
		client: client,
	}, nil
}

func (c *Client) FetchPullRequests() ([]*github.PullRequest, error) {
	return nil, nil
}

func (c *Client) FetchReviews() ([]*github.PullRequestReview, error) {
	return nil, nil
}

func (c *Client) FetchApprovals() ([]*github.PullRequestReview, error) {
	return nil, nil
}

func (c *Client) FetchFollowers() ([]*github.User, error) {
	return nil, nil
}

// todo rename pkg + delete this once i have the overall plugin manager (which will hold any clients)
func main() {
	ctx := context.Background()
	githubClient, err := NewGithubClient(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	prs, err := githubClient.FetchPullRequests()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(prs)
}
