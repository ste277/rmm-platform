package rules

type Classification string

const (
	ClassCompliant           Classification = "compliant"
	ClassExempted            Classification = "exempted"
	ClassBlockedPrerequisite Classification = "blocked_prerequisite"
	ClassBlockedPolicy       Classification = "blocked_policy"
	ClassNeedsReview         Classification = "needs_review"
	ClassNonCompliant        Classification = "non_compliant"
)
