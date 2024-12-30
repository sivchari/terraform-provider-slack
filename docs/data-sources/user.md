---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "slack_user Data Source - terraform-provider-slack"
subcategory: ""
description: |-
  
---

# slack_user (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `email` (String)

### Optional

- `two_factor_type` (String)

### Read-Only

- `deleted` (Boolean)
- `has_2fa` (Boolean)
- `has_files` (Boolean)
- `id` (String) The ID of this resource.
- `is_admin` (Boolean)
- `is_app_user` (Boolean)
- `is_bot` (Boolean)
- `is_invited_user` (Boolean)
- `is_owner` (Boolean)
- `is_primary_owner` (Boolean)
- `is_restricted` (Boolean)
- `is_stranger` (Boolean)
- `is_ultra_restricted` (Boolean)
- `locale` (String)
- `name` (String)
- `presence` (String)
- `real_name` (String, Sensitive)
- `team_id` (String)