package provider

import (
	"context"
	"encoding/json"
	"fmt"

	dtrack "github.com/DependencyTrack/client-go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &workloadIdentityProviderResource{}
	_ resource.ResourceWithConfigure   = &workloadIdentityProviderResource{}
	_ resource.ResourceWithImportState = &workloadIdentityProviderResource{}
)

type (
	workloadIdentityProviderResource struct {
		client *dtrack.Client
		semver *Semver
	}

	workloadIdentityProviderResourceModel struct {
		ID                     types.String `tfsdk:"id"`
		Name                   types.String `tfsdk:"name"`
		Type                   types.String `tfsdk:"type"`
		Issuer                 types.String `tfsdk:"issuer"`
		Audience               types.String `tfsdk:"audience"`
		JWKSURL                types.String `tfsdk:"jwks_url"`
		JWKS                   types.String `tfsdk:"jwks"`
		JWKSKeyIDs             types.List   `tfsdk:"jwks_key_ids"`
		SessionLifetimeSeconds types.Int32  `tfsdk:"session_lifetime_seconds"`
		CreatedAt              types.Int64  `tfsdk:"created_at"`
	}
)

func NewWorkloadIdentityProviderResource() resource.Resource {
	return &workloadIdentityProviderResource{}
}

func (*workloadIdentityProviderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_identity_provider"
}

func (*workloadIdentityProviderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Workload Identity Provider, which DependencyTrack trusts to issue tokens for Workload Identity Federation. Requires API >= 5.2.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier for the Workload Identity Provider. Equal to the Name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the Workload Identity Provider. Must start with an alphanumeric character, " +
					"and contain only alphanumeric characters, underscores, and hyphens, up to 63 characters. Immutable.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Description: "Type of issuer the Workload Identity Provider trusts. One of OIDC, SPIFFE. Immutable.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{stringvalidator.OneOf("OIDC", "SPIFFE")},
			},
			"issuer": schema.StringAttribute{
				Description: "Expected `iss` claim for OIDC providers, or the SPIFFE trust domain for SPIFFE providers, such as `example.org`.",
				Required:    true,
			},
			"audience": schema.StringAttribute{
				Description: "Value that the token's `aud` claim must contain. Use a value unique to the DependencyTrack instance, such as its URL.",
				Required:    true,
			},
			"jwks_url": schema.StringAttribute{
				Description: "HTTPS URL to fetch the signing keys from. For SPIFFE providers, the trust domain's bundle endpoint. " +
					"When neither `jwks_url` nor `jwks` is set on an OIDC provider, DependencyTrack resolves the URL from the issuer's " +
					"OpenID Connect discovery document, and it is exposed here. Conflicts with `jwks`. " +
					"Removing a previously set value does not trigger re-discovery, unless `issuer` changes.",
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("jwks")),
				},
			},
			"jwks": schema.StringAttribute{
				Description: "JSON encoded RFC 7517 JSON Web Key Set holding the issuer's public signing keys, for issuers DependencyTrack cannot reach. " +
					"Inline keys are never refreshed, so update this when the issuer rotates its keys. Not returned by DependencyTrack, so it is " +
					"not populated on import. See `jwks_key_ids` for the stored Key IDs. Conflicts with `jwks_url`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("jwks_url")),
				},
			},
			"jwks_key_ids": schema.ListAttribute{
				Description: "Key IDs of the inline JSON Web Key Set. Absent when the keys are fetched from a URL.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"session_lifetime_seconds": schema.Int32Attribute{
				Description: "Lifetime in seconds of sessions created through the Workload Identity Provider, from 60 to 86400. Defaults to 3600.",
				Optional:    true,
				Computed:    true,
			},
			"created_at": schema.Int64Attribute{
				Description: "Timestamp of when the Workload Identity Provider was created, in milliseconds since the Unix epoch.",
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *workloadIdentityProviderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workloadIdentityProviderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	providerReq := dtrack.CreateWorkloadIdentityProviderRequest{
		Name:                   plan.Name.ValueString(),
		Type:                   dtrack.WorkloadIdentityProviderType(plan.Type.ValueString()),
		Issuer:                 plan.Issuer.ValueString(),
		Audience:               plan.Audience.ValueString(),
		JWKSURL:                "",  // Conditionally Set Below.
		JWKS:                   nil, // Conditionally Set Below.
		SessionLifetimeSeconds: nil, // Conditionally Set Below.
	}
	if !plan.JWKSURL.IsUnknown() && !plan.JWKSURL.IsNull() {
		providerReq.JWKSURL = plan.JWKSURL.ValueString()
	}
	if !plan.JWKS.IsUnknown() && !plan.JWKS.IsNull() {
		jwks, jwksDiag := parseJWKS(plan.JWKS, LifecycleCreate, path.Root("jwks"))
		if jwksDiag != nil {
			resp.Diagnostics.Append(jwksDiag)
			return
		}
		providerReq.JWKS = jwks
	}
	if !plan.SessionLifetimeSeconds.IsUnknown() && !plan.SessionLifetimeSeconds.IsNull() {
		providerReq.SessionLifetimeSeconds = plan.SessionLifetimeSeconds.ValueInt32Pointer()
	}

	tflog.Debug(ctx, "Creating Workload Identity Provider", map[string]any{
		"name": providerReq.Name,
		"type": string(providerReq.Type),
	})

	_, err := r.client.WorkloadIdentityProvider.Create(ctx, providerReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Workload Identity Provider",
			"Error in: "+providerReq.Name+", in original error: "+err.Error(),
		)
		return
	}

	fetched, err := r.client.WorkloadIdentityProvider.Get(ctx, providerReq.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error while creating Workload Identity Provider",
			"Error reading after creation in: "+providerReq.Name+", in original error: "+err.Error(),
		)
		return
	}
	newState := newWorkloadIdentityProviderModel(fetched, plan.JWKS, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Created Workload Identity Provider", map[string]any{
		"id":   newState.ID.ValueString(),
		"name": newState.Name.ValueString(),
	})
}

func (r *workloadIdentityProviderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Fetch state.
	var state workloadIdentityProviderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh.
	name := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Workload Identity Provider", map[string]any{
		"name": name,
	})

	provider, err := r.client.WorkloadIdentityProvider.Get(ctx, name)
	if err != nil {
		err := err.Error()
		if err == "{\"type\":\"about:blank\",\"title\":\"Not Found\",\"detail\":\"The requested resource could not be found.\",\"status\":404} (status: 404)" {
			tflog.Warn(ctx, "Unable to read missing Workload Identity Provider. Will be removed from state.", map[string]any{
				"name": name,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error when reading Workload Identity Provider.",
			"Error in provider with name: "+name+", in original error: "+err,
		)
		return
	}

	newState := newWorkloadIdentityProviderModel(provider, state.JWKS, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update state.
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Read Workload Identity Provider", map[string]any{
		"id":   newState.ID.ValueString(),
		"name": newState.Name.ValueString(),
	})
}

func (r *workloadIdentityProviderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Load plan, config and state.
	var plan workloadIdentityProviderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	var config workloadIdentityProviderResourceModel
	diags = req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	var state workloadIdentityProviderResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Map TF to SDK. Only fields which are provided are changed, so key source fields are only sent when configured.
	name := state.ID.ValueString()
	providerReq := dtrack.UpdateWorkloadIdentityProviderRequest{
		Issuer:                 plan.Issuer.ValueStringPointer(),
		Audience:               plan.Audience.ValueStringPointer(),
		JWKSURL:                nil, // Conditionally Set Below.
		JWKS:                   nil, // Conditionally Set Below.
		SessionLifetimeSeconds: nil, // Conditionally Set Below.
	}
	if !config.JWKSURL.IsNull() && !plan.JWKSURL.IsUnknown() {
		providerReq.JWKSURL = plan.JWKSURL.ValueStringPointer()
	}
	if !config.JWKS.IsNull() && !plan.JWKS.IsUnknown() {
		jwks, jwksDiag := parseJWKS(plan.JWKS, LifecycleUpdate, path.Root("jwks"))
		if jwksDiag != nil {
			resp.Diagnostics.Append(jwksDiag)
			return
		}
		providerReq.JWKS = jwks
	}
	if !plan.SessionLifetimeSeconds.IsUnknown() && !plan.SessionLifetimeSeconds.IsNull() {
		providerReq.SessionLifetimeSeconds = plan.SessionLifetimeSeconds.ValueInt32Pointer()
	}

	// Execute.
	tflog.Debug(ctx, "Updating Workload Identity Provider", map[string]any{
		"name": name,
	})
	err := r.client.WorkloadIdentityProvider.Update(ctx, name, providerReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update Workload Identity Provider",
			"Error in: "+name+", from: "+err.Error(),
		)
		return
	}
	provider, err := r.client.WorkloadIdentityProvider.Get(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when reading Workload Identity Provider.",
			"Error in provider with name: "+name+", in original error: "+err.Error(),
		)
		return
	}

	newState := newWorkloadIdentityProviderModel(provider, plan.JWKS, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update state.
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated Workload Identity Provider", map[string]any{
		"id":   newState.ID.ValueString(),
		"name": newState.Name.ValueString(),
	})
}

func (r *workloadIdentityProviderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Load state.
	var state workloadIdentityProviderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Execute.
	name := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Workload Identity Provider", map[string]any{
		"name": name,
	})
	err := r.client.WorkloadIdentityProvider.Delete(ctx, name)
	if err != nil {
		err := err.Error()
		if err == "{\"type\":\"about:blank\",\"title\":\"Not Found\",\"detail\":\"The requested resource could not be found.\",\"status\":404} (status: 404)" {
			tflog.Warn(ctx, "Unable to delete missing Workload Identity Provider. Ignoring since is desired state.", map[string]any{
				"name": name,
			})
			return
		}
		resp.Diagnostics.AddError(
			"Unable to delete Workload Identity Provider",
			"Unexpected error when trying to delete Workload Identity Provider: "+name+", from error: "+err,
		)
		return
	}
	tflog.Debug(ctx, "Deleted Workload Identity Provider", map[string]any{
		"name": name,
	})
}

func (*workloadIdentityProviderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing Workload Identity Provider", map[string]any{
		"id": req.ID,
	})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Imported Workload Identity Provider", map[string]any{
		"id": req.ID,
	})
}

func (r *workloadIdentityProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	clientInfoData, ok := req.ProviderData.(clientInfo)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Configure Type",
			fmt.Sprintf("Expected provider.clientInfo, got %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = clientInfoData.client
	r.semver = clientInfoData.semver
}

// Maps the API response to the Terraform model. The JWKS is not returned by the API, so is passed through from plan / state.
func newWorkloadIdentityProviderModel(
	provider dtrack.WorkloadIdentityProvider,
	jwks types.String,
	diags *diag.Diagnostics,
) workloadIdentityProviderResourceModel {
	model := workloadIdentityProviderResourceModel{
		ID:                     types.StringValue(provider.Name),
		Name:                   types.StringValue(provider.Name),
		Type:                   types.StringValue(string(provider.Type)),
		Issuer:                 types.StringValue(provider.Issuer),
		Audience:               types.StringValue(provider.Audience),
		JWKSURL:                types.StringNull(), // Conditionally Set Below.
		JWKS:                   jwks,
		JWKSKeyIDs:             types.ListNull(types.StringType), // Conditionally Set Below.
		SessionLifetimeSeconds: types.Int32Value(provider.SessionLifetimeSeconds),
		CreatedAt:              types.Int64Value(provider.CreatedAt),
	}
	if provider.JWKSURL != "" {
		model.JWKSURL = types.StringValue(provider.JWKSURL)
	}
	if provider.JWKSKeyIDs != nil {
		keyIDs, listDiags := types.ListValue(types.StringType, Map(provider.JWKSKeyIDs, func(keyID string) attr.Value {
			return types.StringValue(keyID)
		}))
		diags.Append(listDiags...)
		model.JWKSKeyIDs = keyIDs
	}
	return model
}

func parseJWKS(value types.String, action LifecycleAction, tfPath path.Path) (json.RawMessage, diag.Diagnostic) {
	raw := []byte(value.ValueString())
	if !json.Valid(raw) {
		errDiag := diag.NewAttributeErrorDiagnostic(
			tfPath,
			fmt.Sprintf("Within %s, unable to parse %s as JSON.", action, tfPath.String()),
			"Expected a JSON encoded JSON Web Key Set, such as from jsonencode().",
		)
		return nil, errDiag
	}
	return json.RawMessage(raw), nil
}
