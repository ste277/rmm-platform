package compliance

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

type Finding struct {
	Category   string
	ResourceID string
	Status     Status
	Reason     string
	ActionHint string
}
