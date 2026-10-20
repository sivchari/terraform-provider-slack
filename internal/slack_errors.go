package internal

import (
	"errors"

	"github.com/slack-go/slack"
)

// isSlackError reports whether err is the Slack API error code.
//
// slack-go returns API errors as slack.SlackErrorResponse values whose Error()
// is the bare code, so plain errors carrying only the code are accepted too.
func isSlackError(err error, code string) bool {
	if err == nil {
		return false
	}
	var value slack.SlackErrorResponse
	if errors.As(err, &value) {
		return value.Err == code
	}
	var pointer *slack.SlackErrorResponse
	if errors.As(err, &pointer) {
		return pointer != nil && pointer.Err == code
	}
	return err.Error() == code
}
