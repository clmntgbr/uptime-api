package schedule

import (
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// buildJob turns a workflow into a self-contained job. Only the globals the
// workflow references are included; secret ones are returned apart to be sealed.
func buildJob(w *domainworkflow.Workflow, globals []variable.Global, runID uuid.UUID, trigger string, scheduledAt, deadline, now time.Time) (contract.Job, map[string]string) {
	referenced := map[string]struct{}{}
	for _, c := range w.Calls {
		for _, name := range c.References() {
			referenced[name] = struct{}{}
		}
	}

	job := contract.Job{
		V:            contract.JobVersion,
		ID:           uuid.NewString(),
		Type:         contract.JobType,
		UserID:       w.UserID.String(),
		WorkflowID:   w.ID.String(),
		WorkflowName: w.Name,
		RunID:        runID.String(),
		Trigger:      trigger,
		ScheduledAt:  scheduledAt,
		Deadline:     deadline,
		TimeoutMS:    int(w.Timeout / time.Millisecond),
		EmittedAt:    now,
	}

	secrets := map[string]string{}
	for _, g := range globals {
		if _, ok := referenced[g.Name]; !ok {
			continue
		}
		if g.Secret {
			secrets[g.Name] = g.Value
			continue
		}
		job.Variables = append(job.Variables, contract.JobVariable{Name: g.Name, Type: string(variable.TypeGlobal), Value: g.Value})
	}
	for _, s := range w.Statics {
		job.Variables = append(job.Variables, contract.JobVariable{Name: s.Name, Type: string(variable.TypeStatic), Value: s.Value})
	}

	for _, c := range w.Calls {
		jc := contract.JobCall{
			ID:        c.ID.String(),
			Order:     c.Position,
			Name:      c.Name,
			Method:    c.Method,
			URL:       c.URL,
			Headers:   c.Headers,
			Query:     c.Query,
			Body:      c.Body,
			TimeoutMS: int(c.Timeout / time.Millisecond),
			VerifyTLS: c.VerifyTLS,
			IPVersion: string(c.IPVersion),
			ProxyURL:  c.ProxyURL,
		}
		for _, e := range c.Extractions {
			jc.Extractions = append(jc.Extractions, contract.JobExtraction{Name: e.Name, Source: string(e.Source), Expression: e.Expression})
		}
		for _, r := range c.Rules {
			jc.Rules = append(jc.Rules, contract.JobRule{
				ID: r.ID.String(), Subject: string(r.Subject), Operator: string(r.Operator), Target: r.Target, Value: r.Value,
			})
		}
		job.Calls = append(job.Calls, jc)
	}
	return job, secrets
}
