package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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

	body, err := marshalManifest(manifest)
	if err != nil {
		return nil, err
	}

	raw, err := c.postManifestMethod(ctx, "apps.manifest.create", url.Values{
		"token":    {token},
		"manifest": {string(body)},
	})
	if err != nil {
		return nil, err
	}

	var resp createAppManifestHTTPResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode apps.manifest.create response: %w", err)
	}

	if !resp.Ok {
		return nil, manifestAPIError("apps.manifest.create", resp.Error, resp.Errors)
	}

	return &resp.CreateResponse, nil
}

// UpdateManifestContext updates an app manifest via a raw HTTP call to
// apps.manifest.update, shadowing the embedded slack.Client method: the
// request body must omit empty manifest objects (see marshalManifest) and
// Slack's validation errors have to reach the caller instead of being reduced
// to a bare error code.
func (c *Client) UpdateManifestContext(ctx context.Context, manifest *slack.Manifest, token, appID string) (*slack.UpdateManifestResponse, error) {
	if token == "" {
		token = c.configToken
	}

	body, err := marshalManifest(manifest)
	if err != nil {
		return nil, err
	}

	raw, err := c.postManifestMethod(ctx, "apps.manifest.update", url.Values{
		"token":    {token},
		"app_id":   {appID},
		"manifest": {string(body)},
	})
	if err != nil {
		return nil, err
	}

	var resp slack.UpdateManifestResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode apps.manifest.update response: %w", err)
	}

	if !resp.Ok {
		return nil, manifestAPIError("apps.manifest.update", resp.Error, resp.Errors)
	}

	return &resp, nil
}

func (c *Client) postManifestMethod(ctx context.Context, method string, values url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+method, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", method, err)
	}
	defer httpResp.Body.Close()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", method, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: unexpected status %d: %s", method, httpResp.StatusCode, raw)
	}

	return raw, nil
}

// manifestAPIError formats a manifest API failure with one pointer: message
// line per validation error, so users can see which part of the manifest
// Slack rejected instead of just an error code.
func manifestAPIError(method, code string, errs []slack.ManifestValidationError) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", method, code)
	for _, e := range errs {
		fmt.Fprintf(&b, "\n%s: %s", e.Pointer, e.Message)
	}
	return errors.New(b.String())
}

// marshalManifest encodes the manifest, dropping objects that encoding/json
// cannot omit because slack.Manifest nests value structs: empty objects
// (recursively), a bot_user carrying only an empty display_name, and an
// interactivity carrying only is_enabled=false. Slack treats an absent key
// and its zero form the same, except for event_subscriptions, where {} is
// rejected whenever Socket Mode is disabled.
func marshalManifest(manifest *slack.Manifest) ([]byte, error) {
	body, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("reparse manifest: %w", err)
	}
	pruneZeroObjects(doc)

	normalized, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized manifest: %w", err)
	}
	return normalized, nil
}

func pruneZeroObjects(doc map[string]any) {
	for key, value := range doc {
		child, ok := value.(map[string]any)
		if !ok {
			continue
		}
		pruneZeroObjects(child)
		if isZeroManifestObject(key, child) {
			delete(doc, key)
		}
	}
}

func isZeroManifestObject(key string, obj map[string]any) bool {
	switch len(obj) {
	case 0:
		return true
	case 1:
		switch key {
		case "interactivity":
			enabled, ok := obj["is_enabled"].(bool)
			return ok && !enabled
		case "bot_user":
			name, ok := obj["display_name"].(string)
			return ok && name == ""
		}
	}
	return false
}
