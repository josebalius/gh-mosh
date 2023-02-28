package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/josebalius/gh-mosh/internal/mosh"
)

func Execute() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appType := mosh.AppTypeClient
	if os.Getenv("SERVER") == "true" {
		appType = mosh.AppTypeServer
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return errors.New("API_KEY is not set")
	}

	remoteAddr := os.Getenv("REMOTE_ADDR")
	if remoteAddr == "" {
		return errors.New("REMOTE_ADDR is not set")
	}

	return mosh.NewApp(apiKey, remoteAddr, appType).Run(ctx)
}
