# Attestations expiring in the next 30 days, soonest first. Without a
# control_id filter this needs CCM read-admin permission.
data "jupiterone_attestations" "expiring_soon" {
  expiring_within_days = 30
  state                = "ACTIVE"
}

# Everything attesting a single control.
data "jupiterone_attestations" "for_control" {
  control_id = jupiterone_control.vendor_review.id
}

# Attestations that have already lapsed. A control relying on one of these is
# no longer covered, and its status reverts to the underlying test result.
data "jupiterone_attestations" "lapsed" {
  state = "EXPIRED"
}

output "renewals_due" {
  value = [
    for a in data.jupiterone_attestations.expiring_soon.attestations : {
      subject    = a.subject
      owner      = a.owner
      expires_on = a.expires_on
    }
  ]
}
