package realtime

const (
	ActionCreated      = "created"
	ActionUpdated      = "updated"
	ActionDeleted      = "deleted"
	ActionStarted      = "started"
	ActionFinished     = "finished"
	ActionOpened       = "opened"
	ActionAcknowledged = "acknowledged"
	ActionResolved     = "resolved"
	ActionReopened     = "reopened"
	ActionImported     = "imported"

	EntityUser     = "user"
	EntityWorkflow = "workflow"
	EntityRun      = "run"
	EntityCall     = "call"
	EntityIncident = "incident"
	EntityLibrary  = "library"
)

func EventType(entity, action string) string {
	return entity + "." + action
}
