package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	iv1 "github.com/llmariner/inference-manager/api/v1"
	"github.com/llmariner/inference-manager/common/pkg/sse"
	"google.golang.org/protobuf/encoding/protojson"
)

// New creates a new client.
func New(endpointURL, modelID, apiKey string, logger logr.Logger) *C {
	return &C{
		endpointURL: endpointURL,
		modelID:     modelID,
		apiKey:      apiKey,
		logger:      logger,
	}
}

// C is a client for LLMariner.
type C struct {
	endpointURL string
	modelID     string
	apiKey      string

	logger logr.Logger
}

// CreateChatCompletion creates a chat completion.
func (c *C) CreateChatCompletion(ctx context.Context, prompt string) (<-chan string, error) {
	ch := make(chan string)
	go func() {
		if err := c.createChatCompletion(prompt, ch); err != nil {
			c.logger.Error(err, "failed to create chat completion")
		}
	}()

	return ch, nil
}

func (c *C) createChatCompletion(prompt string, ch chan<- string) error {
	defer func() {
		close(ch)
	}()

	req := &iv1.CreateChatCompletionRequest{
		Model: c.modelID,
		Messages: []*iv1.CreateChatCompletionRequest_Message{
			{
				Content: prompt,
				Role:    "user",
			},
		},
		Stream: true,
	}
	c.logger.Info("Sending a chat completion request", "model", c.modelID, "prompt", prompt)
	body, err := c.sendRequest(http.MethodPost, "/chat/completions", req)
	if err != nil {
		return err
	}

	c.logger.Info("Receiving chat completions")

	scanner := sse.NewScanner(body)

	for scanner.Scan() {
		resp := scanner.Text()
		if !strings.HasPrefix(resp, "data: ") {
			// TODO(kenji): Handle other case.
			continue
		}

		respD := resp[5:]
		if respD == " [DONE]" {
			break
		}

		var d iv1.ChatCompletionChunk
		if err := json.Unmarshal([]byte(respD), &d); err != nil {
			return fmt.Errorf("unmarshal response: %s", err)
		}
		cs := d.Choices
		if len(cs) > 0 {
			// TODO(kenji): Handle multiple choices.
			ch <- cs[0].Delta.Content
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (c *C) sendRequest(
	method string,
	path string,
	req any,
) (io.ReadCloser, error) {
	m := newMarshaler()

	reqBody, err := m.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %s", err)
	}

	var params map[string]interface{}

	hreq, err := http.NewRequest(method, c.endpointURL+path, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %s", err)
	}

	query := hreq.URL.Query()
	for key, value := range params {
		query.Add(key, fmt.Sprintf("%v", value))
	}
	hreq.URL.RawQuery = query.Encode()

	hreq.Header.Add("Authorization", "Bearer "+c.apiKey)
	hreq.Header.Add("Content-Type", "application/json")
	hreq.Header.Add("Accept", "application/json")
	hresp, err := http.DefaultClient.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("send request: %s", err)
	}

	if hresp.StatusCode != http.StatusOK {
		defer func() {
			_ = hresp.Body.Close()
		}()
		s := extractErrorMessage(hresp.Body)
		return nil, fmt.Errorf("unexpected status code: %s (message: %q)", hresp.Status, s)
	}

	return hresp.Body, nil
}

func extractErrorMessage(body io.ReadCloser) string {
	b, err := io.ReadAll(body)
	if err != nil {
		return ""
	}
	type errMessage struct {
		Message string `json:"message"`
	}
	type resp struct {
		// Message is the message from the server. This format is used for gRPC.
		Message string `json:"message"`
		// Error is the error message from the server. This format is used for Ollama.
		Error errMessage `json:"error"`
	}
	var r resp
	if err := json.Unmarshal(b, &r); err != nil {
		// Return the original body if it's not JSON (error response from an non-gRPC HTTP handler).
		return string(b)
	}
	if m := r.Error.Message; m != "" {
		return m
	}
	return r.Message
}

func newMarshaler() *runtime.JSONPb {
	return &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}
}
