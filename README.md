# Terraform Provider JupiterOne

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 1.0.1
- [Go](https://golang.org/doc/install) 1.18 (to build the provider plugin)

## Example Usage

See the `examples` directory

## Building The Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider with `make build` or invoke `go install` directly.

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

## Using the provider

If you're building the provider, follow the instructions to [install it as a plugin.](https://www.terraform.io/docs/plugins/basics.html#installing-a-plugin) After placing it into your plugins directory, run `terraform init` to initialize it.

## Developing the Provider

### Building

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (please check the [requirements](https://github.com/jupiterone/terraform-provider-jupiterone#requirements) before proceeding). To compile the provider, run `make build`.

### Adding or Updating GraphQL Queries

The GraphQL client methods are generated using the
[khan/genqlient](https://github.com/Khan/genqlient) library. The primary
advantages are:

- Compile time query checking
- Generated full types for all API calls

Requirements:

- `node` and `yarn` are installed

```shell
$EDITOR jupiterone/internal/client/<query source file>.graphql

# the J1 env variables and node library is only necessary to query and
# save the updated schema the first time
export JUPITERONE_ACCOUNT=:your_account_id
export JUPITERONE_API_KEY=:your_api_key
export JUPITERONE_REGION=us
yarn add graphql

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

### Using development environment provider locally

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
    "registry.terraform.io/hashicorp/jupiterone" = "/home/$USER/go/bin"
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
