package plugins

import (
	"context"

	"github.com/infocus7/dashie/pkg/plugins/github"
)

type PluginManager struct {
	githubClient *github.Client
}

func NewPluginManager(ctx context.Context) (*PluginManager, error) {
	ghClient, err := github.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &PluginManager{
		githubClient: ghClient,
	}, nil
}
