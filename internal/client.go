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
		return nil, &appmanifest.Error{Method: "apps.manifest.create", Code: resp.Error, Errors: resp.Errors}
	}

	return &resp.CreateResponse, nil
}

// UpdateAppManifest replaces an app manifest via a raw HTTP call to
// apps.manifest.update. The document is the exported manifest with the
// managed fields overlaid (see mergeManifest) and is sent as-is: it carries
// fields this provider does not model, so nothing here may prune it.
// Slack's validation errors reach the caller instead of being reduced to a
// bare error code.
func (c *Client) UpdateAppManifest(ctx context.Context, manifest appmanifest.Document, token, appID string) (*slack.UpdateManifestResponse, error) {
	if token == "" {
		token = c.configToken
	}

	body, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
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
		return nil, &appmanifest.Error{Method: "apps.manifest.update", Code: resp.Error, Errors: resp.Errors}
	}

	return &resp, nil
}

type exportAppManifestHTTPResponse struct {
	Manifest appmanifest.Document `json:"manifest"`
	Ok       bool                 `json:"ok"`
	Error    string               `json:"error"`
}

// ExportAppManifest exports an app manifest as a generic JSON document via
// a raw HTTP call to apps.manifest.export. slack.Client.ExportManifestContext
// decodes into slack.Manifest, which drops every field that struct does not
// model; the document keeps them so an update can send them back.
func (c *Client) ExportAppManifest(ctx context.Context, token, appID string) (appmanifest.Document, error) {
	if token == "" {
		token = c.configToken
	}

	raw, err := c.postManifestMethod(ctx, "apps.manifest.export", url.Values{
		"token":  {token},
		"app_id": {appID},
	})
	if err != nil {
		return nil, err
	}

	var resp exportAppManifestHTTPResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode apps.manifest.export response: %w", err)
	}

	if !resp.Ok {
		return nil, &appmanifest.Error{Method: "apps.manifest.export", Code: resp.Error}
	}

	return resp.Manifest, nil
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

// marshalManifest encodes the manifest for apps.manifest.create, dropping
// objects that encoding/json cannot omit because slack.Manifest nests value
// structs: empty objects (recursively), a bot_user carrying only an empty
// display_name, and an interactivity carrying only is_enabled=false. Slack
// treats an absent key and its zero form the same, except for
// event_subscriptions, where {} is rejected whenever Socket Mode is disabled.
func marshalManifest(manifest *slack.Manifest) ([]byte, error) {
	doc, err := appmanifest.NewDocument(manifest)
	if err != nil {
		return nil, err
	}
	pruneZeroObjects(doc)

	body, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	return body, nil
}

// pruneZeroObjects is only safe on a document built from the plan: an
// exported manifest may carry meaningful empty objects outside the schema.
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

// isZeroManifestObject matches on the key name alone, so it must only see
// objects on schema-managed paths.
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
