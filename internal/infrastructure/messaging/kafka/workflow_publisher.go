package kafka

import (
	"context"
	"encoding/json"

	"go-api/internal/contract"
	"go-api/internal/domain/port"
)

// WorkflowPublisher publishes jobs and results keyed by workflow_id, so that
// all the messages of a workflow stay ordered on one partition.
type WorkflowPublisher struct {
	producer *Producer
}

var (
	_ port.JobPublisher    = (*WorkflowPublisher)(nil)
	_ port.ResultPublisher = (*WorkflowPublisher)(nil)
	_ port.CancelPublisher = (*WorkflowPublisher)(nil)
)

func NewWorkflowPublisher(producer *Producer) *WorkflowPublisher {
	return &WorkflowPublisher{producer: producer}
}

func (p *WorkflowPublisher) PublishJob(ctx context.Context, job contract.Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicWorkflowJobs, []byte(job.WorkflowID), body, map[string]string{
		"type": contract.JobType, "run_id": job.RunID,
	})
}

func (p *WorkflowPublisher) PublishResult(ctx context.Context, result contract.Result) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicWorkflowResults, []byte(result.WorkflowID), body, map[string]string{
		"type": contract.ResultType, "run_id": result.RunID,
	})
}

func (p *WorkflowPublisher) PublishCallProgress(ctx context.Context, progress contract.CallProgress) error {
	body, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicWorkflowResults, []byte(progress.WorkflowID), body, map[string]string{
		"type": contract.CallProgressType, "run_id": progress.RunID,
	})
}

func (p *WorkflowPublisher) PublishCancel(ctx context.Context, cancel contract.Cancel) error {
	body, err := json.Marshal(cancel)
	if err != nil {
		return err
	}
	return p.producer.Produce(ctx, TopicWorkflowCancels, []byte(cancel.RunID), body, map[string]string{
		"type": contract.CancelType, "run_id": cancel.RunID,
	})
}
