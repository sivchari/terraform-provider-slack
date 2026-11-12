# Changelog

## [v0.9.1](https://github.com/sivchari/terraform-provider-slack/compare/v0.9.0...v0.9.1) - 2026-11-12

- ci: publish the release after GoReleaser uploads by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/30

## [v0.9.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.8.0...v0.9.0) - 2026-11-11

- use Go 1.26 by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/25
- build: stop pinning GOTOOLCHAIN in make testacc by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/26
- fix(conversation): detect drift, exclude the bot from members, add rename and destroy options by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/20
- fix(app): tolerate app_not_found on destroy and warn when scopes change by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/23
- feat(client): retry Slack calls that return HTTP 429 after Retry-After by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/24
- fix(usergroup): read remote state, fix import and enabled handling by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/21
- fix(datasource): return on lookup errors and add id, name and handle lookups by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/22
- feat: Manage features.unfurl_domains by @itkq in https://github.com/sivchari/terraform-provider-slack/pull/27
- ci: release with tagpr and GoReleaser v2 by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/28

## [v0.8.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.7.1...v0.8.0) - 2026-09-16

- Add provider api_url attribute to redirect all Slack API calls by @yuya-takeyama in https://github.com/sivchari/terraform-provider-slack/pull/19

## [v0.7.1](https://github.com/sivchari/terraform-provider-slack/compare/v0.7.0...v0.7.1) - 2026-09-09

- slack_app: preserve manifest fields outside the schema on update by @yuya-takeyama in https://github.com/sivchari/terraform-provider-slack/pull/17
- slack_app: derive zero-form pruning from slack.Manifest and test manifestGroups sync by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/18

## [v0.7.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.6.0...v0.7.0) - 2026-09-03

- Fall back to SLACK_TOKEN / SLACK_APP_CONFIGURATION_TOKEN env vars by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/11
- Fix slack_app update, manifest serialization, plan noise, and implement Read by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/12

## [v0.6.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.5.0...v0.6.0) - 2026-09-01

- Make provider token attribute optional by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/9

## [v0.5.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.4.0...v0.5.0) - 2026-08-28

- Add slack_app and slack_app_config_token resources by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/7
- Skip windows/arm build dropped by recent Go by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/8

## [v0.4.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.3.0...v0.4.0) - 2025-01-10

- add resource coversation by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/6

## [v0.3.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.2.0...v0.3.0) - 2025-01-07

- add conversation datasource by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/5

## [v0.2.0](https://github.com/sivchari/terraform-provider-slack/compare/v0.1.0...v0.2.0) - 2025-01-06

- add usergroup resource/datasource by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/4

## [v0.1.0](https://github.com/sivchari/terraform-provider-slack/commits/v0.1.0) - 2024-12-31

- add datasource user by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/1
- add goreleaser by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/2
- release cd by @sivchari in https://github.com/sivchari/terraform-provider-slack/pull/3
