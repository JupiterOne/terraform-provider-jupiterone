# CCM Coverage for the JupiterOne Terraform Provider

**Date:** 2026-08-19
**Status:** Implemented
**Branch:** `feat/ccm-coverage`

> The design reasoning below is kept as written, with an **Implementation
> findings** section at the end recording where the API turned out not to match
> it. One planned item, multi-query control tests, was abandoned on evidence.
>
> This file lives in `design/` rather than `docs/` because `tfplugindocs` owns
> `docs/` and deletes anything it did not generate.

## Context

CCM coverage in this provider is not greenfield. A first phase landed in April 2026
under CCMR-472 (PRs #249, #250), shipping four resources:

| Resource | Fields covered |
|---|---|
| `jupiterone_control_framework` | `name`, `description`, `resource_group_id`, `owner` |
| `jupiterone_control_framework_requirement` | `title`, `framework_id` (RequiresReplace), `description`, `identifier`, `priority`, `section` |
| `jupiterone_control` | `name`, `description`, `resource_group_id`, `state`, `identifier`, `catalog`, `owner`, `remediation`, `exception_process`, `requirement_ids` |
| `jupiterone_control_test` | `name`, `control_id`, `description`, `query`, `results_are` |

The CCM API has moved on since. This design closes the gap to the public subgraph as
it stands on `apps` `main` today.

> **Naming trap.** `jupiterone_framework`, `jupiterone_frameworkitem`, and
> `jupiterone_libraryitem` are the *legacy* Compliance API (`ComplianceFramework`,
> `compliance.graphql`). They are unrelated to CCM and are not touched by this work.

### Source of truth

`apps/packages/apps-api/src/api/graphql/public/{control,controlTest,framework,requirement,attestation}/typeDefs.ts`,
stitched in `public/index.ts` via `buildSubgraphSchema`.

## Scope

### In scope

1. **`jupiterone_control`** — add `mitre_technique`.
2. ~~**`jupiterone_control_test`** — add a repeatable `queries` block supporting multiple
   queries and per-query `description`; deprecate the single-query `query` /
   `results_are` fields without breaking them.~~
   **Abandoned.** CCM control tests are single-query at the service layer; the
   GraphQL list is a facade. Shipped `query_name` instead. See
   [Implementation findings](#multi-query-control-tests-do-not-exist--item-abandoned).
3. **`jupiterone_control_attestation`** — new resource. Create/update, with destroy
   mapped to `revokeAttestation`.
4. **CCM data sources** — expose observed evaluation state, which managed resources
   deliberately do not carry.
5. Docs, examples, and cassette-backed acceptance tests for everything above.

### Out of scope, deliberately

| Dropped | Reason |
|---|---|
| `importFramework`, `importableFrameworks` | Fire-and-forget async job. Returns `{success: true}` with no framework ID, so a resource would have to poll and match on `sourceId`. Worse, it materialises hundreds of requirements/controls/tests that no Terraform resource owns — yet `deleteFramework` destroys them, so `destroy` would delete far more than it created. Not a declarative fit. |
| `evaluateControl`, `evaluateControlTest` | Imperative one-shot actions with no desired state to converge on. Evaluation runs on the CCM cadence regardless; the new data sources surface the resulting status. |
| `restoreControl`, `restoreRequirement`, `restoreControlTest`, `restoreFramework` | Recovery operations, not desired state. Soft-delete behaviour is documented instead (see below). |
| `exportFrameworkJson` | Export utility, same non-declarative shape as import. |

These can be added later; nothing here blocks them.

## Key API constraints discovered

These shaped the design and are easy to get wrong.

### There is no `attestation(id)` query

The attestation schema exposes only `attestations(input: AttestationQueryInput)` and
`orphanedAttestations`. There is no lookup by ID. Consequences:

- `Read` must call `attestations(controlId: <stored>, includeDeleted: true)` and match
  by ID client-side.
- `control_id` is therefore **load-bearing in state**, not merely a create argument.
- `ImportState` cannot be a bare ID. It takes a composite address
  `<control_id>:<attestation_id>`.

### Revocation is terminal

`updateAttestation` has no `revoked` field, so there is no un-revoke. A destroyed
attestation persists with `state = REVOKED`. `Read` treats a revoked attestation as
gone (`RemoveResource`) so Terraform recreates rather than trying to adopt a corpse.

### Deletes across CCM are soft deletes

`deleteControl` and friends set `deleted: true`. The `control(id)` query excludes
deleted records and raises `AppNotFoundError`, so the existing `Read` drift detection
is correct. But a `destroy` followed by `apply` creates a **new** object and leaves the
old one archived. This is documented, not worked around.

### Entitlement gating

Every CCM resolver is wrapped in `withCCMAccess` (`FeatureEntitlementName.CCM`). An
unentitled account receives 403 on all CCM operations, including introspection of the
CCM types.

### Live control limit

`LIVE_CONTROLS_LIMIT` is a usage entitlement enforced by `assertLiveControlsLimit`.
Creating a control with `state = "LIVE"`, or transitioning one to `LIVE`, can fail with
*"Live controls limit reached."* The provider surfaces this as a distinct, actionable
diagnostic rather than a raw GraphQL error blob.

### Requirements belong to exactly one framework

`ControlRequirement.frameworkIds` is a list in the schema, but the resolvers always
populate it as `[frameworkInfo.frameworkId]` — a single element. The existing single
`framework_id` with `RequiresReplace` is already correct; no change needed.

## Design

### `jupiterone_control` — `mitre_technique`

A plain optional string, validated against the MITRE technique shape
(`T####` with an optional `.###` sub-technique, e.g. `T1078`, `T1059.001`).

Both `CreateControlInput` and `UpdateControlInput` accept it, and `UpdateControlInput`
documents `null` to clear — so the field follows the existing optional-string pattern
in this resource. The `GetControlById` selection set gains `mitreTechnique` so drift is
detected.

### `jupiterone_control_test` — multi-query

> **Not implemented.** The premise below is wrong: the API accepts a list but
> persists only the first element. Kept for the record — see
> [Implementation findings](#multi-query-control-tests-do-not-exist--item-abandoned).

The API takes `queries: [ControlTestQueryInput!]!`. Commit `912e76b` deliberately
flattened this to a single top-level `query` / `results_are` pair. Widening it without
breaking existing configs:

- Add an optional repeatable `queries` block: `name`, `query`, `results_are`,
  `description`.
- Keep `query` / `results_are` working, marked `Deprecated`.
- The two forms are **mutually exclusive**, enforced by a config-level validator that
  errors clearly rather than silently preferring one.
- When the legacy form is used, it maps to a single-element `queries` list whose `name`
  matches the test name, preserving today's behaviour exactly.

```hcl
resource "jupiterone_control_test" "mfa" {
  name       = "MFA enforcement"
  control_id = jupiterone_control.mfa.id

  queries {
    name        = "users without mfa"
    query       = "FIND User WITH mfaEnabled = false"
    results_are = "BAD"
    description = "Any result is a finding"
  }

  queries {
    name        = "admins with mfa"
    query       = "FIND User WITH admin = true AND mfaEnabled = true"
    results_are = "GOOD"
  }
}
```

### `jupiterone_control_attestation` — new resource

| Attribute | Notes |
|---|---|
| `control_id` | Required, `RequiresReplace`. Load-bearing for `Read`. |
| `subject` | Required. |
| `description` | Optional. |
| `expires_on` | Required. RFC3339 string; see below. |
| `owner` | Optional, user email. |
| `document_link` | Optional. Validated `http`/`https` only, matching server-side validation. |
| `author` | Computed. Set server-side from request context, never sent. |
| `state` | **Not exposed.** Derived (`ACTIVE`/`EXPIRED`/`REVOKED`); belongs in the data source. |

**`expires_on` representation.** The API takes `expiresOn: Float!` as epoch
milliseconds. Epoch-millisecond floats are a poor authoring experience in HCL, so the
provider accepts RFC3339 and converts.

The naive version of this creates a permanent diff: a user writing
`2027-01-01T00:00:00+01:00` would get `2026-12-31T23:00:00Z` back, and config would
never equal state. So `Read` compares **semantically** — it parses both the stored
string and the API value and only overwrites state when the instants genuinely differ.
The user's literal string is preserved, and real out-of-band changes are still caught.

Destroy calls `revokeAttestation`. Because that is irreversible, the resource
documentation says so plainly.

### Data sources

Managed resources stay purely declarative. Observed state lives here.

| Data source | Purpose |
|---|---|
| `jupiterone_control` | One control by `id` or `source_id`, with `status`, `effective_status`, `has_valid_attestation`, `last_evaluated_on`, `configured`, `number_of_tests`, `framework_ids`, `mitre_technique`. |
| `jupiterone_controls` | Filtered list. Exposes the useful subset of `ControlFilterInput`: `search_text`, `status`, `states`, `catalogs`, `framework_id`, `owner`, `has_valid_attestation`, `mitre_techniques`, `effectiveness`, `ou`. |
| `jupiterone_control_test` | One test by `id`, with `status`, `last_evaluated_on`, `referenced_rule_id`, and per-query `status`, `record_count`, `effective`. |
| `jupiterone_control_tests` | Tests for a `control_id`. |
| `jupiterone_control_frameworks` | Frameworks with their requirements (`id`, `title`, `identifier`). |
| `jupiterone_control_framework_stats` | The scorecard: control and requirement counts, passing/failing, `number_of_attested_controls`, and `section_stats`. |
| `jupiterone_attestations` | Filtered by `control_id`, `owner`, `state`, `expiring_within_days`, with derived `state`. |

`jupiterone_control_frameworks` is not incidental. Because framework import is out of
scope, reading an existing UCF/J1 framework's requirement IDs is the **only** way to
attach Terraform-managed controls to a catalog framework. It closes the loop that
dropping import would otherwise open.

Terraform namespaces data sources separately from resources, so `data.jupiterone_control`
coexisting with `resource.jupiterone_control` is fine.

## Testing

Follow the established harness: `setupTestClientsWithReplaySupport`, one cassette per
test under `jupiterone/cassettes/`, `CheckDestroy` verifying the object is really gone.

| Test | Covers |
|---|---|
| `TestControl_Basic` (extend) | `mitre_technique` create, update, and clear |
| `TestControlTest_Basic` (extend) | Legacy single-query form still applies cleanly |
| `TestControlTest_MultiQuery` | New `queries` block, multiple queries, update |
| `TestControlTest_QueryFormConflict` | Both forms set produces a clear error, no API call |
| `TestControlAttestation_Basic` | Create, update, destroy-revokes |
| `TestControlAttestation_RevokedIsRecreated` | Out-of-band revoke is detected as drift |
| Data source tests | One per data source |

Cassette replay works offline. Recording requires a CCM-entitled account.

## Prerequisites

`jupiterone/internal/client/schema.graphql` is gitignored and generated by
`scripts/get_current_schema.bash`, which introspects a live endpoint. genqlient codegen
depends on it, so **two steps require a CCM-entitled account and API key**:

```bash
export JUPITERONE_ACCOUNT_ID=... JUPITERONE_API_KEY=... JUPITERONE_REGION=...

make jupiterone/internal/client/schema.graphql   # introspect
make generate-client                             # regenerate generated.go
TF_ACC=1 make cassettes                          # record cassettes
```

Everything else — GraphQL operations, resource and data source implementations, docs,
examples, and test bodies — is authored without credentials. GraphQL operations are
syntax-checked ahead of codegen against a scratch schema derived from the `apps`
typeDefs, which catches field-name errors early.

---

## Implementation findings

Recorded during implementation against a CCM-entitled dev account. Each of these
changed the design, and all but the last were caught by acceptance tests rather
than by reading the schema.

### Multi-query control tests do not exist — item abandoned

The design proposed a repeatable `queries` block on the strength of
`CreateControlTestInput.queries: [ControlTestQueryInput!]!`. That list is a
facade.

`gqlToControlTestCreateSchema` reads `gqlInput.queries[0]` and discards the rest.
The service model underneath is singular — `query`, `queryName`, `resultsAre` —
backed by one rule holding exactly one question query, and the read serializer
rebuilds a single-element list from those scalar fields. Per-query `description`
is ignored entirely; the resolver uses the test-level `description`.

An acceptance test proved it: the provider sent two queries, the API returned
one, and `terraform refresh` produced a permanent diff trying to re-add the
second.

A `queries` block would therefore have silently dropped configuration. The
earlier flattening in `912e76b` was correct, not a limitation, so the single
`query` / `results_are` form was restored and left un-deprecated.

What did close a real gap is `query_name`: the API stores the query's name
separately from the test's name, and the provider previously forced them to be
equal.

**If multi-query is ever wanted, it is an API change first**, not a provider
change.

### Attestations require a LIVE control

`createAttestation` rejects any control not in state `LIVE`, because compliance
status is a LIVE-only concept. Documented on the resource, since the requirement
is invisible in the GraphQL schema.

### Optional attributes must be omitted, not sent empty

The recurring defect in this area. Several CCM inputs are optional but reject
empty strings, so a Terraform attribute left unset cannot be sent as `""`:

| Field | Constraint | Fix |
|---|---|---|
| `Control.mitreTechnique` | `/^T\d{4}(\.\d{3})?$/`; `null` unsets, absent preserves | `pointer: true`, no `omitempty`, so clearing sends `null` |
| `Attestation.owner` | validated as an email | `pointer: true` |
| `Attestation.documentLink` | validated as an http(s) URL | `pointer: true` |
| `Requirement.identifier` / `priority` / `section` | `.min(1)` when present | `omitempty` |

The requirement fields were a **pre-existing bug**, not new work:
`jupiterone_control_framework_requirement` declares them optional but always
sent them, so omitting `identifier` failed with "Identifier cannot be empty".
Fixed here because the CCM data source example could not otherwise be written.

Clearing an optional requirement field back to unset is still not possible — the
API keeps the previous value when a field is absent. That limitation predates
this work and is unchanged.

### `omitempty` does not omit a struct

`ControlsQueryInput.sort` is a struct whose two fields are non-nullable enums.
Go's `omitempty` has no effect on structs, so an unset sort serialised as
`{field: "", order: ""}` and failed enum validation on both. It needs
`pointer: true` as well.

### genqlient `for:` directives need a multi-line operation

With genqlient 0.5.0, a `# @genqlient(for: ...)` comment above an operation whose
variables are declared on one line fails with *"for is only applicable to
operations and arguments"*. Splitting the variable list across lines, matching
the existing operations in this repo, resolves it. Worth knowing before
debugging a confusing error.

### List data sources need an `id`

`terraform-plugin-testing` requires an `id` attribute on everything in state.
The list data sources carry a generated one, following the existing
`jupiterone_j1ql_result` precedent.

### The cassette matcher is order-sensitive

`setupCassettes` matches a request to a recorded interaction on HTTP method and
host only, and consumes interactions in order. It never looks at the body.

Terraform reads independent data sources **in parallel**, so a test reading
several of them records in one order and replays in another. The symptom is
confusing: the test passes while recording against the real API and fails on
replay with values that look like another data source's response.

`TestCCMDataSources_Basic` chains its data sources with `depends_on` to force a
deterministic request order. That is a workaround. The durable fix is to match
on the request body — the GraphQL `operationName` and variables — which would
make the whole suite order-independent. Left alone here to avoid changing shared
test infrastructure that 30-plus existing cassettes depend on.

### Recording a cassette can disclose account data

`controlFrameworks` accepts only `cursor` and `includeDeleted`. There is no way
to narrow it, so `jupiterone_control_frameworks` always returns every framework
in the account together with all of their requirements.

Recording an acceptance test against it produced an **8 MB cassette** — ninety
times larger than any existing one — containing internal framework names and
descriptions and the email addresses of six colleagues. This repository is
public, so that cassette would have published dev-account content.

`jupiterone_control_frameworks` is therefore covered by unit tests rather than by
a cassette. The data source itself is unaffected and remains the intended way to
reference a framework the provider does not manage.

**Check the size and contents of any new cassette before committing it.** An
unfiltered list query is the thing to watch for.

### Documentation generation in a worktree

`tfplugindocs` infers the provider name from the working directory, which in a
git worktree is the worktree's name. Run it as:

```bash
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs \
  generate -provider-name terraform-provider-jupiterone
```

Without the flag it matches nothing and empties `docs/`. The flag value must
stay `terraform-provider-jupiterone` to match the `page_title` in the existing
committed docs.
