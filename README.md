# Terraform Provider JupiterOne

**NOTE: This project is currently in beta and is _not_ ready for production use.**

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 1.0.1
- [Go](https://golang.org/doc/install) 1.16 (to build the provider plugin)

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

```
go get github.com/author/dependency
go mod tidy
```

## Using the provider

If you're building the provider, follow the instructions to [install it as a plugin.](https://www.terraform.io/docs/plugins/basics.html#installing-a-plugin) After placing it into your plugins directory, run `terraform init` to initialize it.

## Developing the Provider

### Building

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (please check the [requirements](https://github.com/jupiterone/terraform-provider-jupiterone#requirements) before proceeding). To compile the provider, run `make build`.

### Testing

In order to test the provider, you can simply run `make test`. Pre-recorded API responses
(cassettes) are run. The cassettes are stored in `jupiterone/cassettes/`.
When tests are modified, the cassettes need to be re-recorded.

_Note:_ Recording cassettes creates/updates/destroys real resources. Never run this on
a production JupiterOne organization.

In order to re-record all cassettes you need to have `JUPITERONE_API_KEY` and `JUPITERONE_ACCOUNT_ID`
for your testing organization in your environment. With that, run `make cassettes`.
If you only need to re-record a subset of your tests, you can run `make cassettes TESTARGS ="-run XXX"`.

To run the full suite of Acceptance tests, run `make testacc`.

_Note:_ Acceptance tests create/update/destroy real resources. Never run this on
a production JupiterOne organization.

```sh
$ make testacc
```

### Documentation

To generate new provider documentation run `make docs`
