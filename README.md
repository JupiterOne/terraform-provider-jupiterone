# Terraform Provider JupiterOne

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 1.0.1
- [Go](https://golang.org/doc/install) 1.18 (to build the provider plugin)

## Using the provider

Add the jupiterone provider to your project's terraform:

```hcl
terraform {
  required_providers {
    jupiterone = {
      source  = "JupiterOne/jupiterone"
      version = "x.x.x" # Replace with desired version
    }
  }
}

provider "jupiterone" {
  # Configuration options
  account_id = "xxxxx"
  api_key = "xxxx"
  region  = "us"
}
```

## Example Usage

See the [examples](./examples) directory

## Building The Provider

1. Install [Go](https://go.dev/doc/install) and `make`
1. Clone the repository
1. Enter the repository directory
1. Build the provider with `make build` or invoke `go install` directly.
1. Install the provider locally as referenced [here](#using-development-environment-provider-locally)

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

## Developing the Provider

If this is your first time developing in go, or developing a terraform provider, it may be wise to do some of the [GO Terraform Plugin Framework tutorial](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider). This is what is used to build this provider.

### Building

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (please check the [requirements](https://github.com/jupiterone/terraform-provider-jupiterone#requirements) before proceeding). To compile the provider, run `make build`.

```shell
make build

# If the above command doesn't work, try the next command in the root directory
go install .
```

### Adding a new resource

#### Create resource file

Start your resource development by adding a `jupiterone/resource_[j1_entity].go` file. You should take a look at another file, such as the `resource_user_group.go` to get an idea of what you need in this file, but we will be going into depth on some of the file contents below.

#### J1EntityResource struct

This type is the base type of the terraform resource you are creating. The functions defined in the rest of the file are added to the interface of this type and enable all further functionality.

You will almost always have the `version` and `qlient` fields in this type. They are initialized in the base provider and added to an instance of your type in the `Configure` method.

```go
type J1EntityResource struct {
	version string
	qlient  graphql.Client
}
```

#### J1EntityModel struct

This is the type that represents the terraform resource's state. Generally this is the go equivalent of your graphql resource. You can see that there are json and tfsdk field name definitions. I am not yet sure what the json fields are used for, but the tfsdk fields are used to map the terraform fields to this go type.

This J1EntityModel is using example fields from the `UserGroupModel`, so you have to modify the fields to represent the entity that you are working with. You will probably have an `Id`, but may not a `Name`, `Description`, etc.

```go
// J1EntityModel is the terraform HCL representation of a user group.
type J1EntityModel struct {
	Id          types.String 							`json:"id,omitempty" tfsdk:"id"`
	Name        types.String 							`json:"groupName,omitempty" tfsdk:"name"`
	Description types.String 							`json:"groupDescription,omitempty" tfsdk:"description"`
	Permissions []string     							`json:"groupAbacPermission,omitempty" tfsdk:"permissions"`
	QueryPolicy []map[string][]string			`json:"groupQueryPolicy,omitempty" tfsdk:"query_policy"`
}
```

#### NewJ1EntityResource function

This function is what is used to make the provider aware of your new resource. This will be added to the `Resources` function in the `provider.go` file.

```go
func NewJ1EntityResource() resource.Resource {
	return &J1EntityResource{}
}
```

#### Metadata function

This function is simply used to define your TypeName for the terraform resource. This is the name that will be used when creating resources in your terraform.

```go
func (*J1EntityResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_j1_entity"
}
```

In terraform the name will be used like this:

```terraform
resource "jupiterone_j1_entity" "j1_entity_1" {...}
```

#### Schema function

This function is used to define the schema and documentation for the terraform people will write to build your resource. The terraform go provider lib will use this schema to parse the consumers terraform, validate it, and map it to the J1EntityResource go type.

What you need in this function is totally dependent on what your entity structure looks like. Take a look at some of the other resource.go files to get an idea of what you may need here.

#### Configure function

This function is used to add the version and qlient to the J1EntityResource. The qlient is then used to make http calls to the J1 graphql api. Your `Configure` method should look much the same as below.

```go
func (r *J1EntityResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(*JupiterOneProvider)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected JupiterOneProvider, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.version = p.version
	r.qlient = p.Qlient
}
```

#### Create function

This function is used to create the actual resource. It parses out the J1EntityModel from terraform plan and you work with that data how ever you need, which is mainly just calling the graphql api to create your resource.

Follow this general structure for your create function and look at other resource.go files to get an idea of what you may need here.

```go
func (r *J1EntityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data J1EntityModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Entity specific work goes here, such as calling a gql endpoint
  // Check other files for specifics
  ...

	if err != nil {
		resp.Diagnostics.AddError("failed to create j1 entity", err.Error())
		return
	}

	data.Id = types.StringValue(gqlResponse.Id)

	tflog.Trace(ctx, "Created j1 entity",
		map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
```

#### Delete function

This function is used to delete your terraform resource. It will generally just call a gql endpoint to do that work.

```go
// Delete implements resource.Resource
func (r *J1EntityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data J1EntityModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

  // This is an example call to gql endpoint to delete an entity. You swap with your implementation
	if _, err := client.DeleteJ1Entity(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to j1 entity", err.Error())
	}
}
```

#### Read function

This function is used to read your entity from the jupiterone api. This helps terraform know if there have been changes made in the jupiterone application that need to be overwritten with update intervention.

```go
func (r *J1EntityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data J1EntityModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

  // Grab your entity from jupiterone gql api and then map it to the J1EntityModel
  // See other resource.go files for examples
	entity, err := client.GetJ1Entity(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
				resp.State.RemoveResource(ctx)
		} else {
				resp.Diagnostics.AddError("failed to get entity", err.Error())
		}
		return
	}

	data.Name = types.StringValue(entity.IamGetGroup.GroupName)
	data.Description = types.StringValue(entity.IamGetGroup.GroupDescription)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
```

#### ImportState function

This function simply tells terraform which property to target for the import state. You will generally just copy this function over.

```go
func (*J1EntityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
```

#### Update function

This function is used to update an entity when terraform finds differences between terraform config and the state of the entity in jupiterone. Your contents will be much like your create function, only you should be calling the update action on the gql api.

```go
// Update implements resource.Resource
func (r *J1EntityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data J1EntityModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

  // Entity specific work goes here, such as calling a gql endpoint
  // Check other files for specifics
  ...

	if err != nil {
		resp.Diagnostics.AddError("failed to update j1 entity", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated j1 entity",
		map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
```

### Adding or Updating GraphQL Queries

The GraphQL client methods are generated using the
[khan/genqlient](https://github.com/Khan/genqlient) library. The primary
advantages are:

- Compile time query checking
- Generated full types for all API calls

#### Requirements:

- `node` and `yarn` are installed

#### Add queries and mutations

You should either update an existing `.graphql` file in the `/jupiterone/internal/client` directory, or create a new one.

If you create a new one, be sure to add the file to the [genqlient.yaml](jupiterone/internal/client/genqlient.yaml) file `operations` section.

#### Set environment variables

You should always generate the gql client from the production api so that you do not include any in-progress work from dev. Set these environment variables before running the next commands.

```shell
export JUPITERONE_ACCOUNT=:your_account_id
export JUPITERONE_API_KEY=:your_api_key
export JUPITERONE_REGION=us
```

#### Generating the client

These commands will generate several files:

- introspection_result.json
- jupiterone/internal/client/schema.graphql
- jupiterone/internal/client/generated.go <-- Only generated file that gets committed to the repository

```shell
scripts/get_current_schema.bash
make generate-client
```

### Testing

In order to test the provider, you can simply run `make testacc`. Pre-recorded
API responses (cassettes) are read in from
[jupiterone/cassettes/\*.yaml](jupiterone/cassettes) files and returned. When
tests are modified, the cassettes need to be re-recorded.

_Note:_ Recording cassettes creates/updates/destroys real resources. Never run this on
a production JupiterOne organization.

In order to record cassettes you need to have `JUPITERONE_API_KEY` and `JUPITERONE_ACCOUNT_ID`
for your testing organization in your environment.

To re-record _all_ cassettes:

```sh
export JUPITERONE_ACCOUNT_ID=your-account-id
export JUPITERONE_API_KEY=xxxxxx
export JUPITERONE_REGION=us
make cassettes
```

If you only need to re-record a subset of your tests, delete the related
cassette file and run the tests as usual. This takes advantage of `go-vcr`s
default [`ModeRecordOnce`](https://pkg.go.dev/gopkg.in/dnaeon/go-vcr.v3@v3.1.2/recorder#Mode)
functionality.

```sh
export JUPITERONE_ACCOUNT_ID=your-account-id
export JUPITERONE_API_KEY=xxxxxx
export JUPITERONE_REGION=us
rm jupiterone/cassettes/:some-test.yaml
make testacc
```

### Debugging HTTP Traffic

To log the HTTP request and response contents, set the `TF_LOG` level to `DEBUG`
or lower:

```shell
export TF_LOG=DEBUG
make testacc
```

## Using development environment provider locally

In order to check changes you made locally to the provider, you can use the binary you just compiled by adding the following
to your `~/.terraformrc` file. This is valid for Terraform 0.14+. Please see
[Terraform's documentation](https://www.terraform.io/docs/cli/config/config-file.html#development-overrides-for-provider-developers)
for more details.

```hcl
provider_installation {

  # Use /home/$USER/go/bin as an overridden package directory
  # for the jupiterone provider. This disables the version and checksum
  # verifications for this provider and forces Terraform to look for the
  # jupiterone provider plugin in the given directory.

  # Replace $USER with your username. On Mac and Linux systems this can be found
  # through running "echo $USER" in your terminal.
	dev_overrides {
    "JupiterOne/jupiterone" = "/Users/$USER/go/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

For information about writing acceptance tests, see the main Terraform [contributing guide](https://github.com/hashicorp/terraform/blob/master/.github/CONTRIBUTING.md#writing-acceptance-tests).

### Releasing the Provider

This repository contains a GitHub Action configured to automatically build and
publish assets for release when a tag is pushed that matches the pattern `v*`
(ie. `v0.1.0`).

A [Goreleaser](https://goreleaser.com/) configuration is provided that produces
build artifacts matching the [layout required](https://www.terraform.io/docs/registry/providers/publishing.html#manually-preparing-a-release)
to publish the provider in the Terraform Registry.

Releases will appear as drafts. Once marked as published on the GitHub Releases page,
they will become available via the Terraform Registry.

### Documentation

To generate new provider documentation run `make docs`
