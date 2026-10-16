package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

// SlackPostMessage is the Web API method the notifier calls.
const SlackPostMessage = "https://slack.com/api/chat.postMessage"

// Slack posts messages with a bot token. Errors never contain the token.
type Slack struct {
	URL    string
	Client *http.Client
}

// Slack error codes are short snake_case words; anything else is not echoed.
var slackCode = regexp.MustCompile(`^[a-z_]{1,64}$`)

func (s Slack) Send(ctx context.Context, token, channel, text string) error {
	body, err := json.Marshal(map[string]string{"channel": channel, "text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return errors.New("slack: invalid URL")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("slack: request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack: HTTP %d", resp.StatusCode)
	}
	var r struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&r); err != nil {
		return errors.New("slack: invalid response")
	}
	if !r.OK {
		if slackCode.MatchString(r.Error) {
			return errors.New("slack: " + r.Error)
		}
		return errors.New("slack: request failed")
	}
	return nil
}
