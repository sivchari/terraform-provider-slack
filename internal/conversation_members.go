package internal

import (
	"context"
	"strings"

	"github.com/slack-go/slack"
)

const (
	// conversationPageSize is the page size for cursor-paginated conversations.* calls.
	conversationPageSize = 200

	// inviteBatchSize is the most users conversations.invite accepts per call.
	inviteBatchSize = 1000
)

// allConversationMembers returns every member of the channel, following the
// conversations.members cursor to the last page.
func allConversationMembers(ctx context.Context, client APIClient, channelID string) ([]string, error) {
	var members []string
	cursor := ""
	for {
		page, next, err := client.GetUsersInConversationContext(ctx, &slack.GetUsersInConversationParameters{
			ChannelID: channelID,
			Cursor:    cursor,
			Limit:     conversationPageSize,
		})
		if err != nil {
			return nil, err
		}
		members = append(members, page...)
		if next == "" {
			return members, nil
		}
		cursor = next
	}
}

// findConversationByName returns the public or private channel named name,
// archived ones included, or nil when there is none.
//
// conversations.list has no server-side name filter, so every page is scanned.
func findConversationByName(ctx context.Context, client APIClient, name string) (*slack.Channel, error) {
	cursor := ""
	for {
		page, next, err := client.GetConversationsContext(ctx, &slack.GetConversationsParameters{
			Cursor: cursor,
			Types:  []string{"public_channel", "private_channel"},
			Limit:  conversationPageSize,
		})
		if err != nil {
			return nil, err
		}
		for i := range page {
			if page[i].Name == name {
				return &page[i], nil
			}
		}
		if next == "" {
			return nil, nil
		}
		cursor = next
	}
}

// mergeMemberOrder returns the remote members in the order of prior, followed
// by members new to prior, so a reordered Slack response is not seen as drift.
//
// botID is only kept when prior already lists it: the provider's own user is
// not managed, but a user who declares it must not see it dropped.
func mergeMemberOrder(prior, remote []string, botID string) []string {
	inRemote := make(map[string]struct{}, len(remote))
	for _, m := range remote {
		inRemote[m] = struct{}{}
	}

	merged := make([]string, 0, len(remote))
	seen := make(map[string]struct{}, len(remote))
	for _, m := range prior {
		if _, ok := inRemote[m]; !ok {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		merged = append(merged, m)
	}
	for _, m := range remote {
		if _, ok := seen[m]; ok || m == botID {
			continue
		}
		seen[m] = struct{}{}
		merged = append(merged, m)
	}
	return merged
}

// diffMembers returns the users to invite and to kick so that current becomes
// desired, leaving botID out of both.
func diffMembers(desired, current []string, botID string) (invite, kick []string) {
	inCurrent := make(map[string]struct{}, len(current))
	for _, m := range current {
		inCurrent[m] = struct{}{}
	}
	inDesired := make(map[string]struct{}, len(desired))
	for _, m := range desired {
		inDesired[m] = struct{}{}
	}

	invited := make(map[string]struct{}, len(desired))
	for _, m := range desired {
		if _, ok := inCurrent[m]; ok || m == botID {
			continue
		}
		if _, ok := invited[m]; ok {
			continue
		}
		invited[m] = struct{}{}
		invite = append(invite, m)
	}
	for _, m := range current {
		if _, ok := inDesired[m]; !ok && m != botID {
			kick = append(kick, m)
		}
	}
	return invite, kick
}

// inviteMembers invites users to the channel in batches that fit
// conversations.invite.
func inviteMembers(ctx context.Context, client APIClient, channelID string, users []string) error {
	for start := 0; start < len(users); start += inviteBatchSize {
		end := min(start+inviteBatchSize, len(users))
		if _, err := client.InviteUsersToConversationContext(ctx, channelID, strings.Join(users[start:end], ",")); err != nil {
			return err
		}
	}
	return nil
}
