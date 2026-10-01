package provider

import (
	"context"
	"fmt"

	dtrack "github.com/DependencyTrack/client-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = &serviceAccountResource{}
	_ resource.ResourceWithConfigure   = &serviceAccountResource{}
	_ resource.ResourceWithImportState = &serviceAccountResource{}
)

type (
	serviceAccountResource struct {
		client *dtrack.Client
		semver *Semver
	}

	serviceAccountResourceModel struct {
		ID        types.String `tfsdk:"id"`
		Name      types.String `tfsdk:"name"`
		Username  types.String `tfsdk:"username"`
		Email     types.String `tfsdk:"email"`
		Suspended types.Bool   `tfsdk:"suspended"`
	}
)

func NewServiceAccountResource() resource.Resource {
	return &serviceAccountResource{}
}

func (*serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (*serviceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Service Account, a non-human user which authenticates with API Keys or Workload Identity Federation. " +
			"Permissions and Team memberships are managed with `dependencytrack_user_permission` and `dependencytrack_user_team`, " +
			"using the `username`. Requires API >= 5.2.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identifier for the Service Account. Equal to the Name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the Service Account, without the reserved `svc:` prefix. Must start with an alphanumeric character, " +
					"and contain only alphanumeric characters and `+=,.:@_-`, up to 59 characters. Immutable.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Username of the Service Account, which is the Name with the `svc:` prefix.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Description: "Email address of the Service Account.",
				Optional:    true,
			},
			"suspended": schema.BoolAttribute{
				Description: "Whether the Service Account is suspended. A suspended Service Account cannot authenticate. Defaults to false.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accountReq := dtrack.CreateServiceAccountRequest{
		Name:  plan.Name.ValueString(),
		Email: plan.Email.ValueString(),
	}

	tflog.Debug(ctx, "Creating Service Account", map[string]any{
		"name": accountReq.Name,
	})

	_, err := r.client.ServiceAccount.Create(ctx, accountReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating Service Account",
			"Error in: "+accountReq.Name+", in original error: "+err.Error(),
		)
		return
	}

	// Suspension is not part of creation, so is applied afterwards when requested.
	if !plan.Suspended.IsUnknown() && !plan.Suspended.IsNull() && plan.Suspended.ValueBool() {
		err = r.client.ServiceAccount.Update(ctx, accountReq.Name, dtrack.UpdateServiceAccountRequest{
			Email:     nil,
			Suspended: plan.Suspended.ValueBoolPointer(),
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error while creating Service Account",
				"Error suspending after creation in: "+accountReq.Name+", in original error: "+err.Error(),
			)
			return
		}
	}

	account, err := r.client.ServiceAccount.Get(ctx, accountReq.Name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error while creating Service Account",
			"Error reading after creation in: "+accountReq.Name+", in original error: "+err.Error(),
		)
		return
	}
	newState := newServiceAccountModel(account, plan.Email)

	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Created Service Account", map[string]any{
		"id":       newState.ID.ValueString(),
		"username": newState.Username.ValueString(),
	})
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Fetch state.
	var state serviceAccountResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh.
	name := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Service Account", map[string]any{
		"name": name,
	})

	account, err := r.client.ServiceAccount.Get(ctx, name)
	if err != nil {
		err := err.Error()
		if err == "{\"type\":\"about:blank\",\"title\":\"Not Found\",\"detail\":\"The requested resource could not be found.\",\"status\":404} (status: 404)" {
			tflog.Warn(ctx, "Unable to read missing Service Account. Will be removed from state.", map[string]any{
				"name": name,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error when reading Service Account.",
			"Error in service account with name: "+name+", in original error: "+err,
		)
		return
	}

	newState := newServiceAccountModel(account, state.Email)

	// Update state.
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Read Service Account", map[string]any{
		"id":       newState.ID.ValueString(),
		"username": newState.Username.ValueString(),
	})
}

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Load plan and state.
	var plan serviceAccountResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	var state serviceAccountResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Map TF to SDK. Omitted fields are retained, and an empty email removes the current address.
	name := state.ID.ValueString()
	accountReq := dtrack.UpdateServiceAccountRequest{
		Email:     nil, // Conditionally Set Below.
		Suspended: nil, // Conditionally Set Below.
	}
	if !plan.Email.IsNull() {
		accountReq.Email = plan.Email.ValueStringPointer()
	} else if !state.Email.IsNull() {
		removed := ""
		accountReq.Email = &removed
	}
	if !plan.Suspended.IsUnknown() && !plan.Suspended.IsNull() {
		accountReq.Suspended = plan.Suspended.ValueBoolPointer()
	}

	// Execute.
	tflog.Debug(ctx, "Updating Service Account", map[string]any{
		"name": name,
	})
	err := r.client.ServiceAccount.Update(ctx, name, accountReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update Service Account",
			"Error in: "+name+", from: "+err.Error(),
		)
		return
	}
	account, err := r.client.ServiceAccount.Get(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error when reading Service Account.",
			"Error in service account with name: "+name+", in original error: "+err.Error(),
		)
		return
	}

	newState := newServiceAccountModel(account, plan.Email)

	// Update state.
	diags = resp.State.Set(ctx, newState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated Service Account", map[string]any{
		"id":       newState.ID.ValueString(),
		"username": newState.Username.ValueString(),
	})
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Load state.
	var state serviceAccountResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Execute.
	name := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Service Account", map[string]any{
		"name": name,
	})
	err := r.client.ServiceAccount.Delete(ctx, name)
	if err != nil {
		err := err.Error()
		if err == "{\"type\":\"about:blank\",\"title\":\"Not Found\",\"detail\":\"The requested resource could not be found.\",\"status\":404} (status: 404)" {
			tflog.Warn(ctx, "Unable to delete missing Service Account. Ignoring since is desired state.", map[string]any{
				"name": name,
			})
			return
		}
		resp.Diagnostics.AddError(
			"Unable to delete Service Account",
			"Unexpected error when trying to delete Service Account: "+name+", from error: "+err,
		)
		return
	}
	tflog.Debug(ctx, "Deleted Service Account", map[string]any{
		"name": name,
	})
}

func (*serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	tflog.Debug(ctx, "Importing Service Account", map[string]any{
		"id": req.ID,
	})
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Imported Service Account", map[string]any{
		"id": req.ID,
	})
}

func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Maps the API response to the Terraform model. An absent email is null, unless it was previously known as empty.
func newServiceAccountModel(account dtrack.ServiceAccount, email types.String) serviceAccountResourceModel {
	model := serviceAccountResourceModel{
		ID:        types.StringValue(account.Name),
		Name:      types.StringValue(account.Name),
		Username:  types.StringValue(account.Username),
		Email:     types.StringNull(), // Conditionally Set Below.
		Suspended: types.BoolValue(account.Suspended),
	}
	if !email.IsNull() || account.Email != "" {
		model.Email = types.StringValue(account.Email)
	}
	return model
}
