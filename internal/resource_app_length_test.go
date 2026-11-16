package internal

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

// appLengthField is a slack_app string attribute with a manifest length
// limit from https://docs.slack.dev/reference/app-manifest.
type appLengthField struct {
	path           string
	minLen, maxLen int
	// render places the value into a slack_app configuration.
	render func(value string) appLengthConfig
}

// appLengthConfig holds the HCL fragments that differ between test cases.
type appLengthConfig struct {
	displayInformation string
	features           string
	settings           string
}

var appLengthFields = []appLengthField{
	{path: "display_information.name", maxLen: 35, render: func(v string) appLengthConfig {
		return appLengthConfig{displayInformation: fmt.Sprintf("name = %q", v)}
	}},
	{path: "display_information.description", maxLen: 140, render: func(v string) appLengthConfig {
		return appLengthConfig{displayInformation: fmt.Sprintf("name = \"test\"\ndescription = %q", v)}
	}},
	{path: "display_information.long_description", minLen: 175, maxLen: 4000, render: func(v string) appLengthConfig {
		return appLengthConfig{displayInformation: fmt.Sprintf("name = \"test\"\nlong_description = %q", v)}
	}},
	{path: "features.bot_user.display_name", maxLen: 80, render: func(v string) appLengthConfig {
		return appLengthConfig{features: fmt.Sprintf("bot_user = { display_name = %q }", v)}
	}},
	{path: "features.shortcuts[0].name", maxLen: 24, render: func(v string) appLengthConfig {
		return appLengthConfig{features: shortcutHCL(v, "cb", "desc")}
	}},
	{path: "features.shortcuts[0].callback_id", maxLen: 255, render: func(v string) appLengthConfig {
		return appLengthConfig{features: shortcutHCL("name", v, "desc")}
	}},
	{path: "features.shortcuts[0].description", maxLen: 150, render: func(v string) appLengthConfig {
		return appLengthConfig{features: shortcutHCL("name", "cb", v)}
	}},
	{path: "features.slash_commands[0].command", maxLen: 32, render: func(v string) appLengthConfig {
		return appLengthConfig{features: slashCommandHCL(fmt.Sprintf("command = %q\ndescription = \"desc\"", v))}
	}},
	{path: "features.slash_commands[0].description", maxLen: 2000, render: func(v string) appLengthConfig {
		return appLengthConfig{features: slashCommandHCL(fmt.Sprintf("command = \"/test\"\ndescription = %q", v))}
	}},
	{path: "features.slash_commands[0].usage_hint", maxLen: 1000, render: func(v string) appLengthConfig {
		return appLengthConfig{features: slashCommandHCL(fmt.Sprintf("command = \"/test\"\ndescription = \"desc\"\nusage_hint = %q", v))}
	}},
	{path: "features.slash_commands[0].url", maxLen: 3000, render: func(v string) appLengthConfig {
		return appLengthConfig{features: slashCommandHCL(fmt.Sprintf("command = \"/test\"\ndescription = \"desc\"\nurl = %q", v))}
	}},
	{path: "features.workflow_steps[0].name", maxLen: 50, render: func(v string) appLengthConfig {
		return appLengthConfig{features: fmt.Sprintf("workflow_steps = [{ name = %q, callback_id = \"cb\" }]", v)}
	}},
	{path: "features.workflow_steps[0].callback_id", maxLen: 50, render: func(v string) appLengthConfig {
		return appLengthConfig{features: fmt.Sprintf("workflow_steps = [{ name = \"step\", callback_id = %q }]", v)}
	}},
	{path: "settings.event_subscriptions.request_url", maxLen: 3500, render: func(v string) appLengthConfig {
		return appLengthConfig{settings: fmt.Sprintf("event_subscriptions = { request_url = %q }", v)}
	}},
	{path: "settings.interactivity.request_url", maxLen: 3500, render: func(v string) appLengthConfig {
		return appLengthConfig{settings: fmt.Sprintf("interactivity = { is_enabled = true, request_url = %q }", v)}
	}},
	{path: "settings.interactivity.message_menu_options_url", maxLen: 3500, render: func(v string) appLengthConfig {
		return appLengthConfig{settings: fmt.Sprintf("interactivity = { is_enabled = true, message_menu_options_url = %q }", v)}
	}},
}

func shortcutHCL(name, callbackID, description string) string {
	return fmt.Sprintf("shortcuts = [{ name = %q, callback_id = %q, description = %q, type = \"global\" }]", name, callbackID, description)
}

func slashCommandHCL(attrs string) string {
	return "slash_commands = [{\n" + attrs + "\n}]"
}

// lengthValue returns a string of n characters. URL and slash command values
// keep their usual shape; the others are Japanese text, whose 3-byte
// characters show the length is not counted in bytes.
func lengthValue(path string, n int) string {
	if strings.HasSuffix(path, ".command") {
		return "/" + strings.Repeat("a", n-1)
	}
	if strings.HasSuffix(path, "url") {
		const prefix = "https://example.com/"
		return prefix + strings.Repeat("a", n-len(prefix))
	}
	return strings.Repeat("あ", n)
}

func testAccAppResourceLengthConfig(c appLengthConfig) string {
	if c.displayInformation == "" {
		c.displayInformation = `name = "test"`
	}
	return providerConfig + fmt.Sprintf(`
resource "slack_app" "test" {
	display_information = {
		%s
	}
	features = {
		%s
	}
	settings = {
		%s
	}
}`, c.displayInformation, c.features, c.settings)
}

// TestAccAppResourceManifestLengthLimits checks that a value outside a
// documented manifest length limit fails at plan time, before Slack sees it,
// and that values at the limits pass. No Slack API call is expected.
func TestAccAppResourceManifestLengthLimits(t *testing.T) {
	t.Parallel()

	for _, f := range appLengthFields {
		t.Run(f.path, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := mock.NewMockAPIClient(ctrl)

			requirement := fmt.Sprintf("at most %d characters", f.maxLen)
			if f.minLen > 0 {
				requirement = fmt.Sprintf("between %d and %d characters", f.minLen, f.maxLen)
			}
			// Terraform wraps the detail and may omit the source line of a
			// long value, so only the detail text is matched.
			wantErr := func(n int) *regexp.Regexp {
				return regexp.MustCompile(fmt.Sprintf(`has %d characters, but Slack accepts\s+%s`,
					n, strings.ReplaceAll(regexp.QuoteMeta(requirement), " ", `\s+`)))
			}

			steps := []resource.TestStep{
				{
					Config:             testAccAppResourceLengthConfig(f.render(lengthValue(f.path, f.maxLen))),
					PlanOnly:           true,
					ExpectNonEmptyPlan: true,
				},
				{
					Config:      testAccAppResourceLengthConfig(f.render(lengthValue(f.path, f.maxLen+1))),
					PlanOnly:    true,
					ExpectError: wantErr(f.maxLen + 1),
				},
			}
			if f.minLen > 0 {
				steps = append(steps,
					resource.TestStep{
						Config:             testAccAppResourceLengthConfig(f.render(lengthValue(f.path, f.minLen))),
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
					},
					resource.TestStep{
						Config:      testAccAppResourceLengthConfig(f.render(lengthValue(f.path, f.minLen-1))),
						PlanOnly:    true,
						ExpectError: wantErr(f.minLen - 1),
					},
				)
			}

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories(client),
				Steps:                    steps,
			})
		})
	}
}
