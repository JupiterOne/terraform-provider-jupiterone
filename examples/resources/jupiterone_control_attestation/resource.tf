# An attestation justifies a control by means other than an automated control
# test: a signed policy, a vendor report, a completed review. The control must
# be LIVE, because compliance status is a LIVE-only concept.
resource "jupiterone_control" "vendor_review" {
  name        = "Third-Party Vendor Security Review"
  description = "Annual security review of critical vendors"
  owner       = "security-team@example.com"
  state       = "LIVE"
}

resource "jupiterone_control_attestation" "vendor_review" {
  control_id  = jupiterone_control.vendor_review.id
  subject     = "FY26 vendor security reviews completed"
  description = "All critical vendors reviewed and signed off by the security team."

  # RFC3339. A value carrying a UTC offset is accepted and is not rewritten on
  # refresh, so "2027-01-31T00:00:00+01:00" stays as written.
  expires_on = "2027-01-31T00:00:00Z"

  owner         = "security-team@example.com"
  document_link = "https://example.com/vendor-reviews/fy26.pdf"
}

# Destroying an attestation revokes it, which cannot be undone: the record
# persists with state = REVOKED and a later apply creates a new attestation.
#
# A valid attestation makes the control read as passing, but it never masks a
# failing control test. Use the jupiterone_controls data source with
# has_valid_attestation to see attestation coverage separately from status.
