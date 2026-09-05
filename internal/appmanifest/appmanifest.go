// Package appmanifest holds types for the parts of the Slack App Manifest
// API that github.com/slack-go/slack does not expose. It is a separate leaf
// package so that internal/mock can reference these types in the generated
// APIClient mock without importing package internal, which would create an
// import cycle with internal's own in-package tests.
package appmanifest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// CreateResponse is the response returned by apps.manifest.create.
// github.com/slack-go/slack v0.15.0 decodes this endpoint into
// slack.ManifestResponse, which drops app_id, credentials and
// oauth_authorize_url.
type CreateResponse struct {
	AppID             string                          `json:"app_id"`
	Credentials       Credentials                     `json:"credentials"`
	OAuthAuthorizeURL string                          `json:"oauth_authorize_url"`
	Errors            []slack.ManifestValidationError `json:"errors"`
}

// Credentials are only returned by apps.manifest.create and cannot be
// retrieved afterwards through the API.
type Credentials struct {
	ClientID          string `json:"client_id"`
	ClientSecret      string `json:"client_secret"`
	VerificationToken string `json:"verification_token"`
	SigningSecret     string `json:"signing_secret"`
}

// Manifest extends slack.Manifest with the manifest groups that
// github.com/slack-go/slack v0.15.0 does not model. Features shadows the
// embedded slack.Manifest.Features (encoding/json picks the shallower field
// for the same JSON key), so the slack-go fields stay reachable through
// promotion while the added ones serialize next to them.
type Manifest struct {
	slack.Manifest
	Features Features `json:"features,omitempty" yaml:"features,omitempty"`
}

// Features extends slack.Features with the Agents & AI Apps assistant view.
type Features struct {
	slack.Features
	AssistantView *AssistantView `json:"assistant_view,omitempty" yaml:"assistant_view,omitempty"`
}

// AssistantView is the features.assistant_view manifest group used by apps
// with Agents & AI Apps enabled.
type AssistantView struct {
	AssistantDescription string            `json:"assistant_description" yaml:"assistant_description"`
	SuggestedPrompts     []SuggestedPrompt `json:"suggested_prompts,omitempty" yaml:"suggested_prompts,omitempty"`
	Actions              []AssistantAction `json:"actions,omitempty" yaml:"actions,omitempty"`
}

// SuggestedPrompt is a hard-coded prompt shown in the assistant container.
type SuggestedPrompt struct {
	Title   string `json:"title" yaml:"title"`
	Message string `json:"message" yaml:"message"`
}

// AssistantAction is an action offered in the assistant container.
type AssistantAction struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// Document is an app manifest kept as generic JSON. apps.manifest.export is
// decoded into it so that fields this provider does not model survive a
// read-merge-write round trip through apps.manifest.update, which replaces
// the whole manifest.
type Document map[string]any

// NewDocument converts a typed manifest into its generic JSON form.
func NewDocument(manifest *Manifest) (Document, error) {
	body, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	var doc Document
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("reparse manifest: %w", err)
	}
	return doc, nil
}

// Manifest decodes the generic JSON form into the typed manifest, dropping
// the fields the typed manifest does not model.
func (d Document) Manifest() (*Manifest, error) {
	body, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest document: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest document: %w", err)
	}
	return &manifest, nil
}

// Error is a failed apps.manifest.* call: the Slack error code plus the
// validation errors Slack attaches to invalid manifests.
type Error struct {
	Method string
	Code   string
	Errors []slack.ManifestValidationError
}

// Error formats the failure with one pointer: message line per validation
// error, so callers can see which part of the manifest Slack rejected instead
// of just an error code.
func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s", e.Method, e.Code)
	for _, v := range e.Errors {
		fmt.Fprintf(&b, "\n%s: %s", v.Pointer, v.Message)
	}
	return b.String()
}
