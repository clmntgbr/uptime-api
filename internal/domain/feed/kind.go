package feed

// Kind is a closed vocabulary of feed event facts (not incident states).
type Kind string

const (
	KindCallFailed  Kind = "call.failed"
	KindCallErrored Kind = "call.errored"
	KindCallSlow    Kind = "call.slow"

	KindRunFailed   Kind = "run.failed"
	KindRunErrored  Kind = "run.errored"
	KindRunSkipped  Kind = "run.skipped"
	KindRunTimedOut Kind = "run.timed_out"

	KindIncidentOpened       Kind = "incident.opened"
	KindIncidentReopened     Kind = "incident.reopened"
	KindIncidentAcknowledged Kind = "incident.acknowledged"
	KindIncidentResolved     Kind = "incident.resolved"

	KindTLSThresholdCrossed    Kind = "tls.threshold_crossed"
	KindTLSExpired             Kind = "tls.expired"
	KindTLSRenewed             Kind = "tls.renewed"
	KindAvailabilityBreached   Kind = "availability.breached"
	KindAvailabilityRecovered  Kind = "availability.recovered"
	KindExpiryThresholdCrossed Kind = "expiry.threshold_crossed"

	KindWorkflowCreated Kind = "workflow.created"
	KindWorkflowUpdated Kind = "workflow.updated"
	KindWorkflowDeleted Kind = "workflow.deleted"
	KindLibraryImported Kind = "library.imported"
)

// Severity ranks feed rows for filters and UI color.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeveritySuccess  Severity = "success"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// IncidentFeedKinds are incident_events.type values projected into the feed API.
var IncidentFeedKinds = []string{"opened", "reopened", "acknowledged", "resolved"}

// IncidentTypeToKind maps DB incident_events.type to feed kind.
func IncidentTypeToKind(typ string) Kind {
	switch typ {
	case "opened":
		return KindIncidentOpened
	case "reopened":
		return KindIncidentReopened
	case "acknowledged":
		return KindIncidentAcknowledged
	case "resolved":
		return KindIncidentResolved
	default:
		return Kind("incident." + typ)
	}
}
