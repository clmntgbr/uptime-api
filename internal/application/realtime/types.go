package realtime

const (
	ActionCreated  = "created"
	ActionUpdated  = "updated"
	ActionDeleted  = "deleted"
	ActionStarted  = "started"
	ActionFinished = "finished"

	EntityUser     = "user"
	EntityWorkflow = "workflow"
	EntityRun      = "run"
	EntityCall     = "call"
)

func EventType(entity, action string) string {
	return entity + "." + action
}
