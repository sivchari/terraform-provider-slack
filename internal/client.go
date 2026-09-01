package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
	"github.com/slack-go/slack"
)

const defaultManifestAPIURL = "https://slack.com/api/"

// Client wraps *slack.Client to additionally support the parts of the Slack
// App Manifest API that github.com/slack-go/slack v0.15.0 does not expose.
type Client struct {
	*slack.Client
	httpClient  *http.Client
	apiURL      string
	configToken string
	hasToken    bool
}

// NewClient builds a Client authenticated with the bot token, optionally
// carrying an app configuration token used as the default for app manifest
// and token rotation calls.
func NewClient(token, configurationToken string) *Client {
	var opts []slack.Option
	if configurationToken != "" {
		opts = append(opts, slack.OptionConfigToken(configurationToken))
	}
	return &Client{
		Client:      slack.New(token, opts...),
		httpClient:  http.DefaultClient,
		apiURL:      defaultManifestAPIURL,
		configToken: configurationToken,
		hasToken:    token != "",
	}
}

// HasBotToken reports whether the client was configured with a bot token.
func (c *Client) HasBotToken() bool {
	return c.hasToken
}

type createAppManifestHTTPResponse struct {
	appmanifest.CreateResponse
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

// CreateAppManifest creates an app from an app manifest via a raw HTTP call
// to apps.manifest.create, since slack.Client.CreateManifestContext discards
// app_id, credentials and oauth_authorize_url.
func (c *Client) CreateAppManifest(ctx context.Context, manifest *slack.Manifest, token string) (*appmanifest.CreateResponse, error) {
	if token == "" {
		token = c.configToken
	}

	body, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	values := url.Values{
		"token":    {token},
		"manifest": {string(body)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"apps.manifest.create", bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build apps.manifest.create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call apps.manifest.create: %w", err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read apps.manifest.create response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apps.manifest.create: unexpected status %d: %s", httpResp.StatusCode, raw)
	}

	var resp createAppManifestHTTPResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode apps.manifest.create response: %w", err)
	}

	if !resp.Ok {
		if len(resp.Errors) > 0 {
			return nil, fmt.Errorf("apps.manifest.create: %s: %+v", resp.Error, resp.Errors)
		}
		return nil, fmt.Errorf("apps.manifest.create: %s", resp.Error)
	}

	return &resp.CreateResponse, nil
}
