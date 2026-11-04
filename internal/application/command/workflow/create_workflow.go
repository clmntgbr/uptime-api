package workflow

import (
	"context"
	"fmt"
	"time"

	domainexpiry "go-api/internal/domain/expiry"
	domainnotify "go-api/internal/domain/notifysettings"
	"go-api/internal/domain/port"
	"go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"

	"github.com/google/uuid"
)

// ExpiryPolicyInput is an optional policy set at workflow creation.
type ExpiryPolicyInput struct {
	Type           domainexpiry.Type
	Enabled        bool
	ThresholdsDays []int
}

type CreateWorkflowCommand struct {
	UserID             uuid.UUID
	CreatorEmail       string
	Definition         domainworkflow.Definition
	ExpiryPolicies     []ExpiryPolicyInput
	// NotificationEmails when non-nil overrides the creator email default (empty = no recipients).
	NotificationEmails *[]string
}

type CreateWorkflowHandler struct {
	repo           domainworkflow.WriteRepository
	states         schedule.Repository
	variables      variable.GlobalRepository
	policies       domainexpiry.PolicyRepository
	notifySettings domainnotify.Repository
	outbox         port.OutboxRepository
	clock          port.Clock
}

func NewCreateWorkflowHandler(
	repo domainworkflow.WriteRepository,
	states schedule.Repository,
	variables variable.GlobalRepository,
	policies domainexpiry.PolicyRepository,
	notifySettings domainnotify.Repository,
	outbox port.OutboxRepository,
	clock port.Clock,
) *CreateWorkflowHandler {
	return &CreateWorkflowHandler{
		repo: repo, states: states, variables: variables,
		policies: policies, notifySettings: notifySettings,
		outbox: outbox, clock: clock,
	}
}

// Handle validates and stores the workflow; its first run is the next interval boundary.
func (h *CreateWorkflowHandler) Handle(ctx context.Context, cmd CreateWorkflowCommand) (*domainworkflow.Workflow, error) {
	globals, err := h.variables.Names(ctx, cmd.UserID)
	if err != nil {
		return nil, fmt.Errorf("load global variables: %w", err)
	}

	normalizedPolicies := make([]domainexpiry.Policy, 0, len(cmd.ExpiryPolicies))
	for _, in := range cmd.ExpiryPolicies {
		thresholds, err := domainexpiry.NormalizePolicyThresholds(in.Enabled, in.ThresholdsDays)
		if err != nil {
			return nil, err
		}
		normalizedPolicies = append(normalizedPolicies, domainexpiry.Policy{
			Type: in.Type, Enabled: in.Enabled, ThresholdsDays: thresholds,
		})
	}

	notifyEmails := domainnotify.NormalizeEmails([]string{cmd.CreatorEmail})
	if cmd.NotificationEmails != nil {
		notifyEmails = domainnotify.NormalizeEmails(*cmd.NotificationEmails)
		if err := domainnotify.ValidateEmails(notifyEmails); err != nil {
			return nil, err
		}
	}

	w, err := domainworkflow.New(cmd.UserID, cmd.Definition, globals)
	if err != nil {
		return nil, err
	}
	next := schedule.Never
	if w.Enabled {
		next, err = domainworkflow.NextRunAt(cmd.Definition, h.clock.Now())
		if err != nil {
			return nil, err
		}
	}
	state := schedule.New(w.ID, next)

	err = h.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Save(txCtx, w); err != nil {
			return err
		}
		if err := h.states.Save(txCtx, state); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, p := range normalizedPolicies {
			p.ID = uuid.New()
			p.UserID = cmd.UserID
			p.WorkflowID = w.ID
			p.UpdatedAt = now
			if err := h.policies.Upsert(txCtx, p); err != nil {
				return fmt.Errorf("save expiry policy %s: %w", p.Type, err)
			}
		}
		if h.notifySettings != nil {
			ns := domainnotify.DefaultSettings(cmd.UserID, w.ID, "")
			ns.Emails = notifyEmails
			ns.UpdatedAt = now
			if err := h.notifySettings.Upsert(txCtx, ns); err != nil {
				return fmt.Errorf("save notification settings: %w", err)
			}
		}
		return h.outbox.StoreEvents(txCtx, w.PullEvents())
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow: %w", err)
	}
	return w, nil
}
