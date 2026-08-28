// Package appmanifest holds types for the parts of the Slack App Manifest
// API that github.com/slack-go/slack does not expose. It is a separate leaf
// package so that internal/mock can reference these types in the generated
// APIClient mock without importing package internal, which would create an
// import cycle with internal's own in-package tests.
package appmanifest

import "github.com/slack-go/slack"

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
