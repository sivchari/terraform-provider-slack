package internal

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// appConfigTokenGetenv reads the initial seed refresh token; overridable in
// tests to avoid os.Setenv, which is incompatible with t.Parallel.
var appConfigTokenGetenv = os.Getenv

var (
	_ resource.Resource                = &ResourceAppConfigToken{}
	_ resource.ResourceWithImportState = &ResourceAppConfigToken{}
	_ resource.ResourceWithConfigure   = &ResourceAppConfigToken{}
	_ resource.ResourceWithModifyPlan  = &ResourceAppConfigToken{}
)

// appConfigTokenRefreshEnvVar seeds refresh_token on Create.
const appConfigTokenRefreshEnvVar = "SLACK_APP_REFRESH_TOKEN"

// appConfigTokenExpiryBuffer is subtracted from the access token expiry
// returned by Slack so rotation happens before the token actually expires.
const appConfigTokenExpiryBuffer = time.Hour

type ResourceAppConfigToken struct {
	client APIClient
}

type ResourceAppConfigTokenState struct {
	ID           types.String `tfsdk:"id"`
	RefreshToken types.String `tfsdk:"refresh_token"`
	Token        types.String `tfsdk:"token"`
	ExpiresAt    types.Int64  `tfsdk:"expires_at"`
}

func NewResourceAppConfigToken() resource.Resource {
	return &ResourceAppConfigToken{}
}

func (r *ResourceAppConfigToken) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_app_config_token", req.ProviderTypeName)
}

func (r *ResourceAppConfigToken) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	res.Schema = schema.Schema{
		Description: "Rotates a Slack app configuration token pair and stores it in state. Seed " +
			"the first refresh token via the SLACK_APP_REFRESH_TOKEN environment variable (or " +
			"`terraform import`); every apply after that rotates automatically before the access " +
			"token expires. Create this resource under a provider instance with no " +
			"app_configuration_token set (e.g. an aliased provider), then feed its token " +
			"attribute into the main provider's app_configuration_token to avoid a dependency " +
			"cycle.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"refresh_token": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"token": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"expires_at": schema.Int64Attribute{
				Computed: true,
			},
		},
	}
}

func (r *ResourceAppConfigToken) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(APIClient)
}

// ModifyPlan forces a rotation whenever the stored access token has passed
// its expiry, by marking the computed attributes unknown so Update runs.
func (r *ResourceAppConfigToken) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, res *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state ResourceAppConfigTokenState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if state.ExpiresAt.IsNull() || time.Now().Unix() < state.ExpiresAt.ValueInt64() {
		return
	}

	res.Diagnostics.Append(res.Plan.SetAttribute(ctx, path.Root("refresh_token"), types.StringUnknown())...)
	res.Diagnostics.Append(res.Plan.SetAttribute(ctx, path.Root("token"), types.StringUnknown())...)
	res.Diagnostics.Append(res.Plan.SetAttribute(ctx, path.Root("expires_at"), types.Int64Unknown())...)
}

func (r *ResourceAppConfigToken) Create(ctx context.Context, _ resource.CreateRequest, res *resource.CreateResponse) {
	seed, err := resolveSeedRefreshToken(appConfigTokenGetenv)
	if err != nil {
		res.Diagnostics.AddError("failed to resolve initial refresh token", err.Error())
		return
	}

	state, err := r.rotate(ctx, seed)
	if err != nil {
		res.Diagnostics.AddError("failed to rotate app configuration token", err.Error())
		return
	}

	diags := res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceAppConfigToken) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state ResourceAppConfigTokenState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceAppConfigToken) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var currentState ResourceAppConfigTokenState
	diags := req.State.Get(ctx, &currentState)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state, err := r.rotate(ctx, currentState.RefreshToken.ValueString())
	if err != nil {
		res.Diagnostics.AddError("failed to rotate app configuration token", err.Error())
		return
	}

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceAppConfigToken) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Slack has no API to revoke an app configuration refresh token; dropping
	// it from state is all that can be done.
}

func (r *ResourceAppConfigToken) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	state, err := r.rotate(ctx, req.ID)
	if err != nil {
		res.Diagnostics.AddError("failed to rotate app configuration token", err.Error())
		return
	}

	diags := res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceAppConfigToken) rotate(ctx context.Context, refreshToken string) (ResourceAppConfigTokenState, error) {
	resp, err := r.client.RotateTokensContext(ctx, "", refreshToken)
	if err != nil {
		return ResourceAppConfigTokenState{}, err
	}

	return ResourceAppConfigTokenState{
		ID:           types.StringValue(resp.TeamId),
		RefreshToken: types.StringValue(resp.RefreshToken),
		Token:        types.StringValue(resp.Token),
		ExpiresAt:    types.Int64Value(int64(resp.ExpiresAt) - int64(appConfigTokenExpiryBuffer.Seconds())),
	}, nil
}

// resolveSeedRefreshToken determines the refresh token used to seed the
// first rotation from the SLACK_APP_REFRESH_TOKEN environment variable.
func resolveSeedRefreshToken(getenv func(string) string) (string, error) {
	if token := getenv(appConfigTokenRefreshEnvVar); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("set the %s environment variable", appConfigTokenRefreshEnvVar)
}
