package compliance

// Status values used in compliance findings.
type Status string

const (
	StatusInstalled         Status = "installed"
	StatusMissing           Status = "missing"
	StatusFailed            Status = "failed"
	StatusBlockedByPolicy   Status = "blocked_by_policy"
	StatusBlockedByPrereq   Status = "blocked_by_prerequisite"
	StatusSourceUnreachable Status = "source_unreachable"
	StatusPendingReboot     Status = "pending_reboot"
	StatusExempted          Status = "exempted"
	StatusNeedsReview       Status = "needs_review"
)

// Finding is the internal representation of a compliance check result.
// For the wire format see api.ComplianceFinding in shared/go/api/models.go.
type Finding struct {
	Category   string
	ResourceID string
	Status     Status
	Reason     string
	ActionHint string
}
