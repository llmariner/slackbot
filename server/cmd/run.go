package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-logr/stdr"
	"github.com/llmariner/slackbot/server/internal/config"
	"github.com/llmariner/slackbot/server/internal/events"
	"github.com/llmariner/slackbot/server/internal/llmclient"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func runCmd() *cobra.Command {
	var path string
	var logLevel int
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Parse(path)
			if err != nil {
				return err
			}
			if err := c.Validate(); err != nil {
				return err
			}
			stdr.SetVerbosity(logLevel)

			if err := run(cmd.Context(), c); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "config", "", "Path to the config file")
	cmd.Flags().IntVar(&logLevel, "v", 0, "Log level")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func run(ctx context.Context, c *config.Config) error {
	logger := stdr.New(log.Default())

	slackToken := os.Getenv("SLACK_TOKEN")
	if slackToken == "" {
		return fmt.Errorf("SLACK_TOKEN is required")
	}
	slackAppToken := os.Getenv("SLACK_APP_TOKEN")
	if slackAppToken == "" {
		return fmt.Errorf("SLACK_APP_TOKEN is required")
	}

	llmarinerAPIKey := os.Getenv("LLMARINER_API_KEY")
	if llmarinerAPIKey == "" {
		return fmt.Errorf("LLMARINER_API_KEY is required")
	}

	client := slack.New(
		slackToken,
		slack.OptionAppLevelToken(slackAppToken),
		slack.OptionDebug(true),
	)

	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		// TODO(kenji): Fix this.
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)
	llmc := llmclient.New(
		c.LLMarinerBaseURL,
		c.ModelID,
		llmarinerAPIKey,
		logger.WithName("llmclient"),
	)
	eventHandler, err := events.NewHandler(
		client,
		socketClient,
		llmc,
		logger.WithName("events"),
	)
	if err != nil {
		return err
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error { return eventHandler.Run(ctx) })
	eg.Go(func() error { return socketClient.Run() })
	return eg.Wait()
}
