package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/dnsimple/dnsimple-go/dnsimple"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/terraform-providers/terraform-provider-dnsimple/internal/framework/common"
	"github.com/terraform-providers/terraform-provider-dnsimple/internal/framework/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &DomainSecondaryZoneResource{}
	_ resource.ResourceWithConfigure   = &DomainSecondaryZoneResource{}
	_ resource.ResourceWithImportState = &DomainSecondaryZoneResource{}
)

func NewDomainSecondaryZoneResource() resource.Resource {
	return &DomainSecondaryZoneResource{}
}

// DomainSecondaryZoneResource defines the resource implementation.
type DomainSecondaryZoneResource struct {
	config *common.DnsimpleProviderConfig
}

// DomainSecondaryZoneResourceModel describes the resource data model.
type DomainSecondaryZoneResourceModel struct {
	Name types.String `tfsdk:"name"`
	ID types.Int64 `tfsdk:"id"`
}

func (r *DomainSecondaryZoneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_secondary_zone"
}

func (r *DomainSecondaryZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "DNSimple domain secondary zone resource",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"id": schema.Int64Attribute{
				// For most of the APIs, domain name is the primary key exposed, rather than the ID
				Computed: true,
			},
		},
	}
}

func (r *DomainSecondaryZoneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(*common.DnsimpleProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *provider.DnsimpleProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.config = config
}

func (r *DomainSecondaryZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *DomainSecondaryZoneResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.config.Client.SecondaryDNS.CreateSecondaryZone(ctx, r.config.AccountID, dnsimple.SecondaryZone{Zone: dnsimple.Zone{Name: data.Name.ValueString()}})
	if err != nil {
		var errorResponse *dnsimple.ErrorResponse
		if errors.As(err, &errorResponse) {
			resp.Diagnostics.Append(utils.AttributeErrorsToDiagnostics(errorResponse)...)
			return
		}

		resp.Diagnostics.AddError(
			"failed to create DNSimple secondary DNS zone",
			err.Error(),
		)
	}

	r.updateModelFromAPIResponse(&response.Data.Zone, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainSecondaryZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DomainSecondaryZoneResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	zoneResponse, err := r.config.Client.Zones.GetZone(ctx, r.config.AccountID, data.Name.ValueString())

	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("failed to read DNSimple zone %q", data.Name),
			err.Error(),
		)
		return
	}

	// Unfortunately, DNSimple secondary DNS API doesn't have a Get API for secondary zones. Best you can do is infer it exists if it's linked to a server,
	// and below we use ListPrimaryServers to do that. This won't handle zones that haven't been linked to a server already though. For simplicity, we'll
	// assume that if we can read the zone from GetZone that's enough to consider the "secondary zone resource" created, when in fact we will lazily create
	// it as needed and be aggressive about "recreating" it when attributes change. That is, we will commit on Import that Create will subsequently succeed
	// by making our handling of the "this zone was already created" error idempotent.

	r.updateModelFromAPIResponse(zoneResponse.Data, data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainSecondaryZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No-op
}

func (r *DomainSecondaryZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op; DNSimple's secondary DNS zone API has no "delete secondary zone" API, only a "delete zone" API
}

func (r *DomainSecondaryZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

func (r *DomainSecondaryZoneResource) updateModelFromAPIResponse(server *dnsimple.Zone, data *DomainSecondaryZoneResourceModel) {
	data.Name = types.StringValue(server.Name)
	data.ID = types.Int64Value(server.ID)
}
