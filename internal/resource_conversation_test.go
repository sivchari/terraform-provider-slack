package internal

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

// testBotID is the user ID auth.test reports for the provider's own token.
const testBotID = "UBOT"

func TestAccConversationResource(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "test", IsPrivate: true}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C1", "test").DoAndReturn(f.setTopic).Times(1)
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C1", "updated").DoAndReturn(f.setTopic).Times(1)
	client.EXPECT().SetPurposeOfConversationContext(gomock.Any(), "C1", "test").DoAndReturn(f.setPurpose).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", "U1,U2").DoAndReturn(f.invite).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", "U3").DoAndReturn(f.invite).Times(1)
	client.EXPECT().KickUserFromConversationContext(gomock.Any(), "C1", "U2").DoAndReturn(f.kick).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "test"
	topic = "test"
	purpose = "test"
	is_private = true
	members = ["U1", "U2"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "id", "C1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "name", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "topic", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "purpose", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "is_private", "true"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "2"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "U1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.1", "U2"),
					resource.TestCheckResourceAttr("slack_conversation.test", "action_on_destroy", "archive"),
					resource.TestCheckResourceAttr("slack_conversation.test", "adopt_existing_channel", "false"),
				),
			},
			{
				ResourceName:      "slack_conversation.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccConversationConfig(`
	name = "test"
	topic = "updated"
	purpose = "test"
	is_private = true
	members = ["U1", "U3"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "topic", "updated"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "2"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "U1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.1", "U3"),
				),
			},
		},
	})
}

func TestAccConversationResourceRemoteDrift(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "drift"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C1", "before").DoAndReturn(f.setTopic).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", "U1").DoAndReturn(f.invite).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "drift"
	topic = "before"
	members = ["U1"]`),
			},
			{
				PreConfig: func() {
					f.mutate(func(f *fakeChannel) {
						f.channel.Name = "drift-renamed"
						f.channel.Topic.Value = "drifted"
						f.members = append(f.members, "U9")
					})
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "name", "drift-renamed"),
					resource.TestCheckResourceAttr("slack_conversation.test", "topic", "drifted"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "2"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "U1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.1", "U9"),
				),
			},
		},
	})
}

func TestAccConversationResourceArchivedRemotely(t *testing.T) {
	t.Parallel()

	testAccConversationRemovedRemotely(t, func(f *fakeChannel) { f.channel.IsArchived = true })
}

func TestAccConversationResourceDeletedRemotely(t *testing.T) {
	t.Parallel()

	testAccConversationRemovedRemotely(t, func(f *fakeChannel) { f.infoErr = slack.SlackErrorResponse{Err: "channel_not_found"} })
}

// testAccConversationRemovedRemotely asserts that a channel which disappears
// from Slack is dropped from state, so the next plan proposes recreating it
// and destroy never reaches conversations.archive.
func testAccConversationRemovedRemotely(t *testing.T, remove func(*fakeChannel)) {
	t.Helper()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "gone"}).
		DoAndReturn(f.create).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`name = "gone"`),
			},
			{
				PreConfig:          func() { f.mutate(remove) },
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: func(s *terraform.State) error {
					if _, ok := s.RootModule().Resources["slack_conversation.test"]; ok {
						return fmt.Errorf("slack_conversation.test still in state, want it removed")
					}
					return nil
				},
			},
		},
	})
}

func TestAccConversationResourceActionOnDestroyNone(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "keep"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), gomock.Any()).Return(nil).Times(0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "keep"
	action_on_destroy = "none"`),
				Check: resource.TestCheckResourceAttr("slack_conversation.test", "action_on_destroy", "none"),
			},
		},
	})
}

func TestAccConversationResourceActionOnDestroyInvalid(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "keep"
	action_on_destroy = "delete"`),
				ExpectError: regexp.MustCompile(`action_on_destroy`),
			},
		},
	})
}

func TestAccConversationResourceAdoptExisting(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{channel: testChannel("C9", "adopted"), members: []string{testBotID, "U5"}}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "adopted"}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).
		Return([]slack.Channel{testChannel("C1", "other"), testChannel("C9", "adopted")}, "", nil).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C9", "U1").DoAndReturn(f.invite).Times(1)
	client.EXPECT().KickUserFromConversationContext(gomock.Any(), "C9", "U5").DoAndReturn(f.kick).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C9").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "adopted"
	adopt_existing_channel = true
	members = ["U1"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "id", "C9"),
					resource.TestCheckResourceAttr("slack_conversation.test", "adopt_existing_channel", "true"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "U1"),
				),
			},
		},
	})
}

func TestAccConversationResourceAdoptArchivedChannel(t *testing.T) {
	t.Parallel()

	archived := testChannel("C9", "adopted")
	archived.IsArchived = true

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "adopted"}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).Return([]slack.Channel{archived}, "", nil).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "adopted"
	adopt_existing_channel = true`),
				ExpectError: regexp.MustCompile(`(?s)archived.*requires\s+a\s+user\s+token`),
			},
		},
	})
}

func TestAccConversationResourceAdoptNotFound(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "adopted"}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).Return(nil, "", nil).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "adopted"
	adopt_existing_channel = true`),
				ExpectError: regexp.MustCompile(`(?s)name_taken.*private\s+channel\s+the\s+bot`),
			},
		},
	})
}

func TestAccConversationResourceNameTakenWithoutAdopt(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "taken"}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccConversationConfig(`name = "taken"`),
				ExpectError: regexp.MustCompile(`name_taken`),
			},
		},
	})
}

func TestAccConversationResourceBotIsNeverInvitedOrKicked(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "bot"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", "U1").DoAndReturn(f.invite).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", "U2").DoAndReturn(f.invite).Times(1)
	// No expectation exists for kicking or inviting testBotID, so gomock fails
	// the test if the provider ever tries to.
	client.EXPECT().KickUserFromConversationContext(gomock.Any(), "C1", "U1").DoAndReturn(f.kick).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "bot"
	members = ["U1"]`),
				Check: resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "1"),
			},
			{
				Config: testAccConversationConfig(`
	name = "bot"
	members = ["U2"]`),
				Check: resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "U2"),
			},
			{
				Config: testAccConversationConfig(`
	name = "bot"
	members = ["U2", "` + testBotID + `"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "2"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.1", testBotID),
				),
			},
		},
	})
}

func TestAccConversationResourceMembersUnmanaged(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	// conversations.members, invite and kick have no expectation: a null
	// members attribute must never touch the member list.
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "free"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C1", "a").DoAndReturn(f.setTopic).Times(1)
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C1", "").DoAndReturn(f.setTopic).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`name = "free"`),
				Check:  resource.TestCheckNoResourceAttr("slack_conversation.test", "members"),
			},
			{
				Config: testAccConversationConfig(`
	name = "free"
	topic = "a"`),
				Check: resource.TestCheckResourceAttr("slack_conversation.test", "topic", "a"),
			},
			{
				Config: testAccConversationConfig(`name = "free"`),
				Check:  resource.TestCheckNoResourceAttr("slack_conversation.test", "topic"),
			},
		},
	})
}

func TestAccConversationResourceMembersPagination(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{pageSize: 2}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "paged"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", "U1,U2,U3,U4").DoAndReturn(f.invite).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				// Five members (the bot and U1..U4) span three pages of two. A
				// reader that stopped at the first page would see a diff here.
				Config: testAccConversationConfig(`
	name = "paged"
	members = ["U1", "U2", "U3", "U4"]`),
				Check: resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "4"),
			},
			{
				ResourceName:      "slack_conversation.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccConversationResourcePrivacyChangeIsRejected(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "privacy"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "privacy"
	is_private = false`),
			},
			{
				Config: testAccConversationConfig(`
	name = "privacy"
	is_private = true`),
				ExpectError: regexp.MustCompile(`(?s)convert\s+channel\s+privacy.*name_taken`),
			},
		},
	})
}

func TestAccConversationResourceRename(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "before"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().RenameConversationContext(gomock.Any(), "C1", "after").DoAndReturn(f.rename).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`name = "before"`),
				Check:  resource.TestCheckResourceAttr("slack_conversation.test", "name", "before"),
			},
			{
				Config: testAccConversationConfig(`name = "after"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "id", "C1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "name", "after"),
				),
			},
		},
	})
}

func TestAccConversationResourceImportDirectMessage(t *testing.T) {
	t.Parallel()

	testAccConversationImportRejected(t, "D1", func(c *slack.Channel) { c.IsIM = true })
}

func TestAccConversationResourceImportMultiPartyDirectMessage(t *testing.T) {
	t.Parallel()

	testAccConversationImportRejected(t, "G1", func(c *slack.Channel) { c.IsMpIM = true })
}

// testAccConversationImportRejected asserts that importing a conversation of
// a kind this resource cannot archive fails before anything is read or changed.
func testAccConversationImportRejected(t *testing.T, id string, kind func(*slack.Channel)) {
	t.Helper()

	f := &fakeChannel{channel: testChannel(id, "")}
	kind(&f.channel)
	client := newConversationClient(t, f)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:        testAccConversationConfig(`name = "dm"`),
				ResourceName:  "slack_conversation.test",
				ImportState:   true,
				ImportStateId: id,
				ExpectError:   regexp.MustCompile(`(?s)public\s+and\s+private\s+channels\s+only`),
			},
		},
	})
}

func TestAccConversationResourceAdoptPublicChannelJoinsTheBot(t *testing.T) {
	t.Parallel()

	adopted := testChannel("C9", "adopted")
	f := &fakeChannel{channel: adopted, members: []string{"U5"}}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "adopted"}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).Return([]slack.Channel{adopted}, "", nil).Times(1)
	join := client.EXPECT().JoinConversationContext(gomock.Any(), "C9").DoAndReturn(f.join).Times(1)
	// Topic, invite and kick need the bot in the channel, so they must follow the join.
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C9", "t").DoAndReturn(f.setTopic).Times(1).After(join)
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C9", "U1").DoAndReturn(f.invite).Times(1).After(join)
	client.EXPECT().KickUserFromConversationContext(gomock.Any(), "C9", "U5").DoAndReturn(f.kick).Times(1).After(join)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C9").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "adopted"
	adopt_existing_channel = true
	topic = "t"
	members = ["U1"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "id", "C9"),
					resource.TestCheckResourceAttr("slack_conversation.test", "topic", "t"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.#", "1"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "U1"),
				),
			},
		},
	})
}

func TestAccConversationResourceAdoptPrivateChannelWithoutMembership(t *testing.T) {
	t.Parallel()

	adopted := testChannel("C9", "adopted")
	adopted.IsPrivate = true
	f := &fakeChannel{channel: adopted, members: []string{"U5"}}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "adopted", IsPrivate: true}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).Return([]slack.Channel{adopted}, "", nil).Times(1)
	// Joining, topic, invite, kick and archive have no expectation: a private
	// channel cannot be joined, and nothing may change before the error.
	client.EXPECT().JoinConversationContext(gomock.Any(), gomock.Any()).Times(0)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), gomock.Any()).Times(0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "adopted"
	is_private = true
	adopt_existing_channel = true
	topic = "t"
	members = ["U1"]`),
				ExpectError: regexp.MustCompile(`(?s)private.*Invite\s+the\s+bot`),
			},
		},
	})
}

func TestAccConversationResourceAdoptEnforcesTopicAndPurpose(t *testing.T) {
	t.Parallel()

	adopted := testChannel("C9", "adopted")
	adopted.Topic.Value = "old"
	adopted.Purpose.Value = "same"
	f := &fakeChannel{channel: adopted, members: []string{testBotID}}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "adopted"}).
		Return(nil, slack.SlackErrorResponse{Err: "name_taken"}).Times(1)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).Return([]slack.Channel{adopted}, "", nil).Times(1)
	// The unset topic overwrites "old"; the purpose already matches, so
	// conversations.setPurpose has no expectation and must not be called.
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C9", "").DoAndReturn(f.setTopic).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C9").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "adopted"
	adopt_existing_channel = true
	purpose = "same"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("slack_conversation.test", "topic"),
					resource.TestCheckResourceAttr("slack_conversation.test", "purpose", "same"),
				),
			},
		},
	})
}

func TestAccConversationResourceDuplicateMembers(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "dup"
	members = ["U1", "U2", "U1"]`),
				ExpectError: regexp.MustCompile(`(?s)"U1"\s+appears\s+more\s+than\s+once`),
			},
		},
	})
}

// TestAccConversationResourceCreateFailureKeepsChannelInState asserts that a
// failure after conversations.create leaves the channel in state as tainted, so
// the final destroy archives it instead of leaving an orphan in Slack.
func TestAccConversationResourceCreateFailureKeepsChannelInState(t *testing.T) {
	t.Parallel()

	f := &fakeChannel{}
	client := newConversationClient(t, f)
	expectMembersRead(client, f)
	client.EXPECT().CreateConversationContext(gomock.Any(), slack.CreateConversationParams{ChannelName: "partial"}).
		DoAndReturn(f.create).Times(1)
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "C1", "t").
		Return(nil, errors.New("topic failed")).Times(1)
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "C1").DoAndReturn(f.archive).Times(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationConfig(`
	name = "partial"
	topic = "t"`),
				ExpectError: regexp.MustCompile(`topic failed`),
			},
		},
	})
}

func testAccConversationConfig(attrs string) string {
	return providerConfig + `
resource "slack_conversation" "test" {
` + attrs + `
}`
}

// newConversationClient returns a mock client whose read-only calls are served
// by f. Calls that change Slack are left to each test so they can state
// exactly how often and with which arguments they are expected.
func newConversationClient(t *testing.T, f *fakeChannel) *mock.MockAPIClient {
	t.Helper()

	client := mock.NewMockAPIClient(gomock.NewController(t))
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().AuthTestContext(gomock.Any()).Return(&slack.AuthTestResponse{UserID: testBotID}, nil).AnyTimes()
	client.EXPECT().GetConversationInfoContext(gomock.Any(), gomock.Any()).DoAndReturn(f.info).AnyTimes()
	return client
}

func expectMembersRead(client *mock.MockAPIClient, f *fakeChannel) {
	client.EXPECT().GetUsersInConversationContext(gomock.Any(), gomock.Any()).DoAndReturn(f.users).AnyTimes()
}

// fakeChannel is the single Slack channel behind a test. The provider runs in
// the test process on other goroutines, hence the mutex.
type fakeChannel struct {
	mu       sync.Mutex
	channel  slack.Channel
	members  []string
	pageSize int
	infoErr  error
}

func (f *fakeChannel) mutate(fn func(*fakeChannel)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func (f *fakeChannel) snapshot() *slack.Channel {
	c := f.channel
	return &c
}

func (f *fakeChannel) create(_ context.Context, params slack.CreateConversationParams) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channel = testChannel("C1", params.ChannelName)
	f.channel.IsPrivate = params.IsPrivate
	f.members = []string{testBotID}
	return f.snapshot(), nil
}

func (f *fakeChannel) info(_ context.Context, in *slack.GetConversationInfoInput) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.infoErr != nil {
		return nil, f.infoErr
	}
	if in.ChannelID != f.channel.ID {
		return nil, slack.SlackErrorResponse{Err: "channel_not_found"}
	}
	return f.snapshot(), nil
}

func (f *fakeChannel) users(_ context.Context, params *slack.GetUsersInConversationParameters) ([]string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	start := 0
	if params.Cursor != "" {
		start, _ = strconv.Atoi(params.Cursor)
	}
	end := len(f.members)
	next := ""
	if f.pageSize > 0 && start+f.pageSize < end {
		end = start + f.pageSize
		next = strconv.Itoa(end)
	}
	return append([]string(nil), f.members[start:end]...), next, nil
}

func (f *fakeChannel) join(context.Context, string) (*slack.Channel, string, []string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members = append(f.members, testBotID)
	return f.snapshot(), "", nil, nil
}

func (f *fakeChannel) setTopic(_ context.Context, _, topic string) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channel.Topic.Value = topic
	return f.snapshot(), nil
}

func (f *fakeChannel) setPurpose(_ context.Context, _, purpose string) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channel.Purpose.Value = purpose
	return f.snapshot(), nil
}

func (f *fakeChannel) rename(_ context.Context, _, name string) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channel.Name = name
	return f.snapshot(), nil
}

func (f *fakeChannel) invite(_ context.Context, _ string, users ...string) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range users {
		f.members = append(f.members, strings.Split(u, ",")...)
	}
	return f.snapshot(), nil
}

func (f *fakeChannel) kick(_ context.Context, _, user string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.members[:0]
	for _, m := range f.members {
		if m != user {
			kept = append(kept, m)
		}
	}
	f.members = kept
	return nil
}

func (f *fakeChannel) archive(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channel.IsArchived = true
	return nil
}
