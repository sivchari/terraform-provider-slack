package internal

import (
	"errors"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/sivchari/terraform-provider-slack/internal/mock"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"
)

func testSlackUserGroups() []slack.UserGroup {
	return []slack.UserGroup{
		{
			ID:          "S1",
			TeamID:      "T1",
			IsUserGroup: true,
			Name:        "Admins",
			Description: "the admins",
			Handle:      "admins",
			IsExternal:  true,
			AutoType:    "admin",
			CreatedBy:   "U1",
			UpdatedBy:   "U2",
			DeletedBy:   "",
			Prefs: slack.UserGroupPrefs{
				Channels: []string{"C1"},
				Groups:   []string{"G1"},
			},
			UserCount: 1,
			Users:     []string{"U3"},
		},
		{
			ID:          "S2",
			TeamID:      "T1",
			IsUserGroup: true,
			Name:        "Retired",
			Handle:      "retired",
			DeletedBy:   "U1",
			DateDelete:  1700000000,
			UserCount:   0,
			Users:       []string{},
		},
	}
}

func expectUserGroupsList(client *mock.MockAPIClient, groups []slack.UserGroup, err error) {
	// The three options are include_count, include_users and include_disabled.
	client.EXPECT().GetUserGroupsContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(groups, err).AnyTimes()
}

func TestAccDataSourceUserGroup_Lookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "by id", config: `id = "S1"`},
		{name: "by name", config: `name = "Admins"`},
		{name: "by handle", config: `handle = "admins"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := mock.NewMockAPIClient(ctrl)
			client.EXPECT().HasBotToken().Return(true).AnyTimes()
			expectUserGroupsList(client, testSlackUserGroups(), nil)

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories(client),
				Steps: []resource.TestStep{
					{
						Config: testAccDataSourceUserGroup(tt.config),
						Check: resource.ComposeAggregateTestCheckFunc(
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "id", "S1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "team_id", "T1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "is_user_group", "true"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "name", "Admins"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "description", "the admins"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "handle", "admins"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "enabled", "true"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "is_external", "true"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "auto_type", "admin"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "created_by", "U1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "updated_by", "U2"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "deleted_by", ""),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "prefs.channels.0", "C1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "prefs.groups.0", "G1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "user_count", "1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "users.#", "1"),
							resource.TestCheckResourceAttr("data.slack_usergroup.test", "users.0", "U3"),
						),
					},
				},
			})
		})
	}
}

func TestAccDataSourceUserGroup_Disabled(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	expectUserGroupsList(client, testSlackUserGroups(), nil)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUserGroup(`handle = "retired"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "id", "S2"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "enabled", "false"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "deleted_by", "U1"),
				),
			},
		},
	})
}

func TestAccDataSourceUserGroup_PrefersEnabledOverDisabled(t *testing.T) {
	t.Parallel()

	groups := []slack.UserGroup{
		{ID: "S1", Name: "Ops", Handle: "ops", DateDelete: 1700000000},
		{ID: "S2", Name: "Ops", Handle: "ops"},
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	expectUserGroupsList(client, groups, nil)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUserGroup(`handle = "ops"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "id", "S2"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "enabled", "true"),
				),
			},
		},
	})
}

func TestAccDataSourceUserGroup_NotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      string
		expectError *regexp.Regexp
	}{
		{name: "id", config: `id = "S404"`, expectError: regexp.MustCompile(`(?s)No\s+user\s+group\s+has\s+the\s+id\s+"S404"`)},
		{name: "name", config: `name = "Nobody"`, expectError: regexp.MustCompile(`(?s)No\s+user\s+group\s+has\s+the\s+name\s+"Nobody"`)},
		{name: "handle", config: `handle = "nobody"`, expectError: regexp.MustCompile(`(?s)No\s+user\s+group\s+has\s+the\s+handle\s+"nobody"`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := mock.NewMockAPIClient(ctrl)
			client.EXPECT().HasBotToken().Return(true).AnyTimes()
			expectUserGroupsList(client, testSlackUserGroups(), nil)

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories(client),
				Steps: []resource.TestStep{
					{
						Config:      testAccDataSourceUserGroup(tt.config),
						ExpectError: tt.expectError,
					},
				},
			})
		})
	}
}

func TestAccDataSourceUserGroup_ListError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	expectUserGroupsList(client, nil, errors.New("missing_scope"))

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceUserGroup(`id = "S1"`),
				ExpectError: regexp.MustCompile(`(?s)Failed\s+to\s+list\s+the\s+Slack\s+user\s+groups.*missing_scope`),
			},
		},
	})
}

func TestAccDataSourceUserGroup_LookupKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "id and name", config: `
	id   = "S1"
	name = "Admins"`},
		{name: "all three", config: `
	id     = "S1"
	name   = "Admins"
	handle = "admins"`},
		{name: "none", config: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := mock.NewMockAPIClient(ctrl)
			client.EXPECT().HasBotToken().Return(true).AnyTimes()

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories(client),
				Steps: []resource.TestStep{
					{
						Config:      testAccDataSourceUserGroup(tt.config),
						ExpectError: regexp.MustCompile(`(?s)exactly\s+one\s+of\s+id,\s+name,\s+handle\s+must\s+be\s+set`),
					},
				},
			})
		})
	}
}

func testAccDataSourceUserGroup(attrs string) string {
	return providerConfig + `
data "slack_usergroup" "test" {
	` + attrs + `
}`
}
