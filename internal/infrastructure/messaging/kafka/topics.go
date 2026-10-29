package kafka

// Topics are created by deploy/kafka/topics.sh; auto-creation is disabled on the broker.
const (
	TopicWorkflowJobs           = "workflow.jobs"
	TopicWorkflowResults        = "workflow.results"
	TopicWorkflowCancels        = "workflow.cancels"
	TopicEvents                 = "events"
	TopicNotificationsRequests  = "notifications.requests"
	TopicNotificationsResults   = "notifications.results"
)

func RetryTopic(topic string) string { return topic + ".retry" }

func DLQTopic(topic string) string { return topic + ".dlq" }
