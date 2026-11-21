package presenter

import (
	"time"

	domainfeed "go-api/internal/domain/feed"

	"github.com/google/uuid"
)

type FeedEventResponse struct {
	ID         uuid.UUID      `json:"id"`
	Kind       string         `json:"kind"`
	Severity   string         `json:"severity"`
	At         time.Time      `json:"at"`
	Title      string         `json:"title"`
	WorkflowID *uuid.UUID     `json:"workflowId,omitempty"`
	RunID      *uuid.UUID     `json:"runId,omitempty"`
	CallID     *uuid.UUID     `json:"callId,omitempty"`
	IncidentID *uuid.UUID     `json:"incidentId,omitempty"`
	Payload    map[string]any `json:"payload"`
}

type FeedListResponse struct {
	Members    []FeedEventResponse `json:"members"`
	NextCursor string              `json:"nextCursor,omitempty"`
}

func NewFeedListResponse(events []domainfeed.Event, nextCursor string) FeedListResponse {
	members := make([]FeedEventResponse, 0, len(events))
	for _, e := range events {
		payload := e.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		members = append(members, FeedEventResponse{
			ID: e.ID, Kind: string(e.Kind), Severity: string(e.Severity),
			At: e.At, Title: e.Title,
			WorkflowID: e.WorkflowID, RunID: e.RunID, CallID: e.CallID, IncidentID: e.IncidentID,
			Payload: payload,
		})
	}
	return FeedListResponse{Members: members, NextCursor: nextCursor}
}
