package internal

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

const (
	testUserGroupAddr   = "slack_usergroup.test"
	testUserGroupTeamID = "T001"
)

// fakeUserGroups emulates the Slack usergroups.* methods for a single group.
type fakeUserGroups struct {
	mu       sync.Mutex
	group    *slack.UserGroup
	seq      int
	listArgs []slack.GetUserGroupsParams
}

func newFakeUserGroups() *fakeUserGroups {
	return &fakeUserGroups{}
}

func (f *fakeUserGroups) copyGroup() slack.UserGroup {
	g := *f.group
	g.Users = append([]string(nil), f.group.Users...)
	g.Prefs.Channels = append([]string(nil), f.group.Prefs.Channels...)
	return g
}

func (f *fakeUserGroups) find(id string) error {
	if f.group == nil || f.group.ID != id {
		return slack.SlackErrorResponse{Err: "no_such_subteam"}
	}
	return nil
}

func (f *fakeUserGroups) create(_ context.Context, in slack.UserGroup) (slack.UserGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.seq++
	g := in
	g.ID = fmt.Sprintf("S%03d", f.seq)
	g.IsUserGroup = true
	if g.Handle == "" {
		g.Handle = "auto-" + strings.ToLower(g.Name)
	}
	if g.TeamID == "" {
		g.TeamID = testUserGroupTeamID
	}
	g.Users = nil
	f.group = &g
	return f.copyGroup(), nil
}

func (f *fakeUserGroups) list(_ context.Context, opts ...slack.GetUserGroupsOption) ([]slack.UserGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var params slack.GetUserGroupsParams
	for _, opt := range opts {
		opt(&params)
	}
	f.listArgs = append(f.listArgs, params)

	if f.group == nil || (f.group.DateDelete != 0 && !params.IncludeDisabled) {
		return nil, nil
	}
	g := f.copyGroup()
	if !params.IncludeUsers {
		g.Users = nil
	}
	return []slack.UserGroup{g}, nil
}

func (f *fakeUserGroups) update(_ context.Context, id string, opts ...slack.UpdateUserGroupsOption) (slack.UserGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.find(id); err != nil {
		return slack.UserGroup{}, err
	}
	var params slack.UpdateUserGroupsParams
	for _, opt := range opts {
		opt(&params)
	}
	if params.Name != "" {
		f.group.Name = params.Name
	}
	if params.Handle != "" {
		f.group.Handle = params.Handle
	}
	if params.Description != nil {
		f.group.Description = *params.Description
	}
	if params.Channels != nil {
		f.group.Prefs.Channels = append([]string(nil), *params.Channels...)
	}
	return f.copyGroup(), nil
}

func (f *fakeUserGroups) setMembers(_ context.Context, id, members string) (slack.UserGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.find(id); err != nil {
		return slack.UserGroup{}, err
	}
	if members == "" {
		return slack.UserGroup{}, slack.SlackErrorResponse{Err: "no_users_provided"}
	}
	f.group.Users = strings.Split(members, ",")
	return f.copyGroup(), nil
}

func (f *fakeUserGroups) enable(_ context.Context, id string) (slack.UserGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.find(id); err != nil {
		return slack.UserGroup{}, err
	}
	f.group.DateDelete = 0
	return f.copyGroup(), nil
}

func (f *fakeUserGroups) disable(_ context.Context, id string) (slack.UserGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.find(id); err != nil {
		return slack.UserGroup{}, err
	}
	f.group.DateDelete = 1700000000
	return f.copyGroup(), nil
}

// remote changes the group behind the provider's back.
func (f *fakeUserGroups) remote(fn func(g *slack.UserGroup)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f.group)
}

func (f *fakeUserGroups) drop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.group = nil
}

func (f *fakeUserGroups) snapshot() slack.UserGroup {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.copyGroup()
}

// userGroupCalls is how many times each mutating Slack method is expected.
type userGroupCalls struct {
	create, update, members, enable, disable int
}

type userGroupExpectations struct {
	create, update, members, enable, disable *gomock.Call
}

// newUserGroupClient wires f behind a mock client; reads are unrestricted
// while every mutating method must be called exactly as many times as want.
func newUserGroupClient(t *testing.T, f *fakeUserGroups, want userGroupCalls) (*mock.MockAPIClient, userGroupExpectations) {
	t.Helper()

	client := mock.NewMockAPIClient(gomock.NewController(t))
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetUserGroupsContext(gomock.Any(), gomock.Any()).DoAndReturn(f.list).AnyTimes()

	return client, userGroupExpectations{
		create:  client.EXPECT().CreateUserGroupContext(gomock.Any(), gomock.Any()).DoAndReturn(f.create).Times(want.create),
		update:  client.EXPECT().UpdateUserGroupContext(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(f.update).Times(want.update),
		members: client.EXPECT().UpdateUserGroupMembersContext(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(f.setMembers).Times(want.members),
		enable:  client.EXPECT().EnableUserGroupContext(gomock.Any(), gomock.Any()).DoAndReturn(f.enable).Times(want.enable),
		disable: client.EXPECT().DisableUserGroupContext(gomock.Any(), gomock.Any()).DoAndReturn(f.disable).Times(want.disable),
	}
}

func testAccUserGroupConfig(body string) string {
	return providerConfig + `
resource "slack_usergroup" "test" {
` + body + `
}`
}

func TestAccUserGroupResourceMissingBotToken(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(false).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: providerConfigNoToken + `
resource "slack_usergroup" "test" {
	name = "test"
}`,
				ExpectError: regexp.MustCompile(`The provider attribute "token" is required to use slack_usergroup`),
			},
		},
	})
}

func TestAccUserGroupResource_Basic(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	// create sets the members once, the update step once more; destroy disables.
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, update: 1, members: 2, disable: 1})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfig(`
	name        = "eng"
	handle      = "eng"
	description = "engineers"
	channels    = ["C1"]
	users       = ["U1", "U2"]
	team_id     = "T001"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "id", "S001"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "name", "eng"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "handle", "eng"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "description", "engineers"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "team_id", "T001"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "channels.#", "1"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "channels.0", "C1"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.#", "2"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.0", "U1"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.1", "U2"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "true"),
				),
			},
			{
				ResourceName:      testUserGroupAddr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccUserGroupConfig(`
	name        = "eng2"
	handle      = "eng2"
	description = "engineers"
	channels    = ["C1"]
	users       = ["U2", "U3"]
	team_id     = "T001"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "name", "eng2"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "handle", "eng2"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.#", "2"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.0", "U2"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.1", "U3"),
				),
			},
		},
	})

	got := f.snapshot()
	if got.Name != "eng2" || got.Handle != "eng2" || strings.Join(got.Users, ",") != "U2,U3" {
		t.Errorf("remote group = %+v, want name/handle eng2 and users U2,U3", got)
	}
	if got.DateDelete == 0 {
		t.Error("remote group is still enabled after destroy, want disabled")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, args := range f.listArgs {
		if !args.IncludeUsers || !args.IncludeDisabled {
			t.Errorf("list call %d = %+v, want include_users and include_disabled", i, args)
		}
	}
}

func TestAccUserGroupResource_CreateDisabled(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	// Destroying an already disabled group must not call disable again.
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, disable: 1})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfig(`
	name    = "retired"
	enabled = false`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "id", "S001"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "false"),
					resource.TestCheckNoResourceAttr(testUserGroupAddr, "users.#"),
				),
			},
		},
	})

	if got := f.snapshot(); got.DateDelete == 0 {
		t.Error("remote group is enabled, want disabled")
	}
}

func TestAccUserGroupResource_RemoteDrift(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, update: 1, members: 2, disable: 1})

	config := testAccUserGroupConfig(`
	name   = "eng"
	handle = "eng"
	users  = ["U1", "U2"]`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					f.remote(func(g *slack.UserGroup) {
						g.Name = "renamed"
						g.Handle = "renamed-handle"
						g.Users = []string{"U9"}
					})
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "name", "renamed"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "handle", "renamed-handle"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.#", "1"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.0", "U9"),
				),
			},
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "name", "eng"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "users.#", "2"),
				),
			},
		},
	})

	if got := f.snapshot(); got.Name != "eng" || got.Handle != "eng" || strings.Join(got.Users, ",") != "U1,U2" {
		t.Errorf("remote group = %+v, want drift reverted", got)
	}
}

func TestAccUserGroupResource_RemovedRemotely(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	// The first group vanishes, so only the recreated one is disabled on destroy.
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 2, members: 2, disable: 1})

	config := testAccUserGroupConfig(`
	name  = "eng"
	users = ["U1"]`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(testUserGroupAddr, "id", "S001"),
			},
			{
				PreConfig:          f.drop,
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: func(s *terraform.State) error {
					if _, ok := s.RootModule().Resources[testUserGroupAddr]; ok {
						return fmt.Errorf("%s is still in state after the group was removed remotely", testUserGroupAddr)
					}
					return nil
				},
			},
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(testUserGroupAddr, "id", "S002"),
			},
		},
	})
}

func TestAccUserGroupResource_DisabledRemotely(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, members: 1, enable: 1, disable: 1})

	config := testAccUserGroupConfig(`
	name  = "eng"
	users = ["U1"]`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					f.remote(func(g *slack.UserGroup) { g.DateDelete = 1700000000 })
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "false"),
			},
			{
				Config: config,
				Check:  resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "true"),
			},
		},
	})

	if got := f.snapshot(); got.DateDelete == 0 {
		t.Error("remote group is enabled after destroy, want disabled")
	}
}

func TestAccUserGroupResource_DisableWithRename(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	client, calls := newUserGroupClient(t, f, userGroupCalls{create: 1, update: 1, disable: 1})
	gomock.InOrder(calls.update, calls.disable)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfig(`name = "before"`),
				Check:  resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "true"),
			},
			{
				Config: testAccUserGroupConfig(`
	name    = "after"
	enabled = false`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "name", "after"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "false"),
				),
			},
		},
	})

	got := f.snapshot()
	if got.Name != "after" || got.DateDelete == 0 {
		t.Errorf("remote group = %+v, want renamed and disabled", got)
	}
}

func TestAccUserGroupResource_EmptyUsersRejected(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, members: 1, disable: 1})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{Config: testAccUserGroupConfig(`
	name  = "eng"
	users = ["U1"]`)},
			{
				Config: testAccUserGroupConfig(`
	name  = "eng"
	users = []`),
				ExpectError: regexp.MustCompile(`no_users_provided`),
			},
		},
	})

	if got := f.snapshot(); strings.Join(got.Users, ",") != "U1" {
		t.Errorf("remote users = %v, want [U1] untouched", got.Users)
	}
}

func TestAccUserGroupResource_ComputedFromSlack(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	// No users are configured, so usergroups.users.update must never run.
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, disable: 1})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupConfig(`name = "Eng"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(testUserGroupAddr, "handle", "auto-eng"),
					resource.TestCheckResourceAttr(testUserGroupAddr, "team_id", testUserGroupTeamID),
					resource.TestCheckResourceAttr(testUserGroupAddr, "description", ""),
					resource.TestCheckResourceAttr(testUserGroupAddr, "enabled", "true"),
					resource.TestCheckNoResourceAttr(testUserGroupAddr, "users.#"),
				),
			},
		},
	})
}

func TestAccUserGroupResource_ImportNotFound(t *testing.T) {
	t.Parallel()

	f := newFakeUserGroups()
	client, _ := newUserGroupClient(t, f, userGroupCalls{create: 1, disable: 1})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{Config: testAccUserGroupConfig(`name = "eng"`)},
			{
				ResourceName:  testUserGroupAddr,
				ImportState:   true,
				ImportStateId: "S999",
				ExpectError:   regexp.MustCompile(`does not exist`),
			},
		},
	})
}

func TestMergeOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prior  []string
		remote []string
		want   []string
	}{
		{name: "same order", prior: []string{"a", "b"}, remote: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "keeps prior order", prior: []string{"b", "a"}, remote: []string{"a", "b"}, want: []string{"b", "a"}},
		{name: "drops removed", prior: []string{"a", "b", "c"}, remote: []string{"c", "a"}, want: []string{"a", "c"}},
		{name: "appends new", prior: []string{"b"}, remote: []string{"a", "b", "c"}, want: []string{"b", "a", "c"}},
		{name: "no prior", remote: []string{"x", "y"}, want: []string{"x", "y"}},
		{name: "empty remote", prior: []string{"a"}, want: []string{}},
		{name: "duplicate prior", prior: []string{"a", "a"}, remote: []string{"a"}, want: []string{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mergeOrder(tt.prior, tt.remote)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") || len(got) != len(tt.want) {
				t.Errorf("mergeOrder(%v, %v) = %v, want %v", tt.prior, tt.remote, got, tt.want)
			}
		})
	}
}
