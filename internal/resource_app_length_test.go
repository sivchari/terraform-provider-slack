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

// appLengthField is a slack_app description attribute with a manifest length
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
}

var appLengthFields = []appLengthField{
	{path: "display_information.description", maxLen: 140, render: func(v string) appLengthConfig {
		return appLengthConfig{displayInformation: fmt.Sprintf("name = \"test\"\ndescription = %q", v)}
	}},
	{path: "display_information.long_description", minLen: 175, maxLen: 4000, render: func(v string) appLengthConfig {
		return appLengthConfig{displayInformation: fmt.Sprintf("name = \"test\"\nlong_description = %q", v)}
	}},
	{path: "features.shortcuts[0].description", maxLen: 150, render: func(v string) appLengthConfig {
		return appLengthConfig{features: fmt.Sprintf(
			"shortcuts = [{ name = \"name\", callback_id = \"cb\", description = %q, type = \"global\" }]", v)}
	}},
	{path: "features.slash_commands[0].description", maxLen: 2000, render: func(v string) appLengthConfig {
		return appLengthConfig{features: fmt.Sprintf("slash_commands = [{ command = \"/test\", description = %q }]", v)}
	}},
}

// lengthValue returns n characters of Japanese text, whose 3-byte
// characters show the length is not counted in bytes.
func lengthValue(n int) string {
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
}`, c.displayInformation, c.features)
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
					Config:             testAccAppResourceLengthConfig(f.render(lengthValue(f.maxLen))),
					PlanOnly:           true,
					ExpectNonEmptyPlan: true,
				},
				{
					Config:      testAccAppResourceLengthConfig(f.render(lengthValue(f.maxLen + 1))),
					PlanOnly:    true,
					ExpectError: wantErr(f.maxLen + 1),
				},
			}
			if f.minLen > 0 {
				steps = append(steps,
					resource.TestStep{
						Config:             testAccAppResourceLengthConfig(f.render(lengthValue(f.minLen))),
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
					},
					resource.TestStep{
						Config:      testAccAppResourceLengthConfig(f.render(lengthValue(f.minLen - 1))),
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
