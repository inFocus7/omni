package main

import (
	"context"
	"os"

	"github.com/infocus7/dashie/pkg/plugins"
	"github.com/infocus7/dashie/ui"
)

func main() {
	ctx := context.Background()
	pm, err := plugins.NewPluginManager(ctx)
	if err != nil {
		panic(err)
	}

	d, err := ui.Dashboard(pm)
	if err != nil {
		panic(err)
	}

	err = d.Execute(os.Stdout, nil)
	if err != nil {
		panic(err)
	}
}
