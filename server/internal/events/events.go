package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/llmariner/slackbot/server/internal/llmclient"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// NewHandler returns a new Handler.
func NewHandler(
	client *slack.Client,
	socketClient *socketmode.Client,
	llmClient *llmclient.C,
	logger logr.Logger,
) (*Handler, error) {
	return &Handler{
		client:       client,
		socketClient: socketClient,
		llmClient:    llmClient,
		logger:       logger,
	}, nil
}

// Handler is an event handler.
type Handler struct {
	client       *slack.Client
	socketClient *socketmode.Client
	llmClient    *llmclient.C
	logger       logr.Logger
}

// Run runs the handler.
func (h *Handler) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("Shutting down socketmode listener.")
			return nil
		case event := <-h.socketClient.Events:
			switch event.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
				if !ok {
					h.logger.Info("Could not type cast the event to the EventsAPIEvent", "event", event)
					continue
				}
				h.socketClient.Ack(*event.Request)

				err := h.handleEventMessage(eventsAPIEvent)
				if err != nil {
					return err
				}
			}
		}
	}
}

// handleEventMessage takes an event and handle it properly based on the type of event.
func (h *Handler) handleEventMessage(event slackevents.EventsAPIEvent) error {
	switch event.Type {
	case slackevents.CallbackEvent:
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			if err := h.handleAppMentionEvent(ev); err != nil {
				return err
			}
		default:
			return errors.New("unsupported data type")
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func (h *Handler) handleAppMentionEvent(event *slackevents.AppMentionEvent) error {
	h.logger.Info("Received an AppMentionEvent", "event", event)

	ch, err := h.llmClient.CreateChatCompletion(context.Background(), event.Text)
	if err != nil {
		return err
	}

	attachment := slack.Attachment{
		Title: "LLMariner Slackbot",
		Text:  "...",
		Color: "#3d3d3d",
	}
	_, timestamp, err := h.client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("post message: %s", err)
	}

	const limit = 100
	count := 0

	var msg string
	for m := range ch {
		msg += m

		count += len(m)
		if count > limit {
			if err := h.updateResponse(event, timestamp, msg); err != nil {
				return err
			}
			count = 0
		}
	}
	msg += "\n"

	if err := h.updateResponse(event, timestamp, msg); err != nil {
		return err
	}

	return nil
}

func (h *Handler) updateResponse(event *slackevents.AppMentionEvent, timestamp string, msg string) error {
	attachment := slack.Attachment{
		Title: "LLMariner Slackbot",
		Text:  msg,
		Color: "#3d3d3d",
	}
	if _, _, _, err := h.client.UpdateMessage(event.Channel, timestamp, slack.MsgOptionAttachments(attachment)); err != nil {
		return fmt.Errorf("update message: %s", err)
	}
	return nil
}
