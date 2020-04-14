# terraform-provider-jupiterone

**NOTE: This project is currently in beta and is _not_ ready for production use.**

Requirements
------------

-	[Terraform](https://www.terraform.io/downloads.html) 0.10.x
-	[Go](https://golang.org/doc/install) 1.11 (to build the provider plugin)

## Example Usage

```terraform
# Configure the JupiterOne provider
provider "jupiterone" {
  api_key = "${var.jupiterone_api_key}"
  account_id = "${var.jupiterone_account}"
}

# Create a new JupiterOne rule
resource "jupiterone_rule" "unencrypted_critical_data_stores" {
  # ...
}
```

Building The Provider
---------------------

Clone repository to: `$GOPATH/src/github.com/jupiterone/terraform-provider-jupiterone`

```sh
$ mkdir -p $GOPATH/src/github.com/jupiterone; cd $GOPATH/src/github.com/jupiterone
$ git clone git@github.com:jupiterone/terraform-provider-jupiterone
```

Enter the provider directory and build the provider

```sh
$ cd $GOPATH/src/github.com/jupiterone/terraform-provider-jupiterone
$ make build
```

Using the provider
----------------------
If you're building the provider, follow the instructions to [install it as a plugin.](https://www.terraform.io/docs/plugins/basics.html#installing-a-plugin) After placing it into your plugins directory,  run `terraform init` to initialize it.

Developing the Provider
---------------------

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (please check the [requirements](https://github.com/jupiterone/terraform-provider-jupiterone#requirements) before proceeding). You'll also need to correctly setup a [GOPATH](http://golang.org/doc/code.html#GOPATH), as well as adding `$GOPATH/bin` to your `$PATH`.

*Note:* This project uses [Go Modules](https://blog.golang.org/using-go-modules) making it safe to work with it outside of your existing [GOPATH](http://golang.org/doc/code.html#GOPATH).

To compile the provider, run `make build`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

```sh
$ make build
...
$ $GOPATH/bin/terraform-provider-jupiterone
...
```

In order to test the provider, you can simply run `make test`. Pre-recorded API responses
(cassettes) are run. The cassettes are stored in `jupiterone/cassettes/`.
When tests are modified, the cassettes need to be re-recorded.

```sh
$ make test
```

*Note:* Recording cassettes creates/updates/destroys real resources. Never run this on
a production JupiterOne organization.

In order to re-record all cassettes you need to have `JUPITERONE_API_KEY` and `JUPITERONE_ACCOUNT_ID`
for your testing organization in your environment. With that, run `make cassettes`.
If you only need to re-record a subset of your tests, you can run `make cassettes TESTARGS ="-run XXX"`.

To run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create/update/destroy real resources. Never run this on
a production JupiterOne organization.

```sh
$ make testacc
```

## Resources

### `jupiterone_rule`

```terraform
resource "jupiterone_rule" "unencrypted_critical_data_stores" {
  name = "unencrypted-critical-data-stores"
  description = "Unencrypted data store with classification label of 'critical' or 'sensitive' or 'confidential' or 'restricted'"
  polling_interval = "ONE_DAY"

  question {
    queries {
      name = "query0"
      query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
      version = "v1"
    }
  }

  outputs = [
    "queries.query0.total",
    "alertLevel"
  ]

  operations = <<EOF
[
  {
    "when": {
      "type": "FILTER",
      "specVersion": 1,
      "condition": "{{queries.query0.total != 0}}"
    },
    "actions": [
      {
        "targetValue": "HIGH",
        "type": "SET_PROPERTY",
        "targetProperty": "alertLevel"
      },
      {
        "type": "CREATE_ALERT"
      }
    ]
  }
]
EOF
}
```

### `jupiterone_question`

```terraform
resource "jupiterone_question" "unencrypted_critical_data_stores" {
  title = "Unencrypted critical data stores"
  description = "Unencrypted data store with classification label of 'critical' or 'sensitive' or 'confidential' or 'restricted'"
  tags = ["hello"]

  query {
    name = "query0"
    query = "Find DataStore with classification=('critical' or 'sensitive' or 'confidential' or 'restricted') and encrypted!=true"
    version = "v1"
  }
}
```
