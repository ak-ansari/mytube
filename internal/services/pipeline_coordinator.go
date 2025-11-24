package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/repository"
)

type PipelineCoordinator struct {
	repo  repository.VideoRepository
	sm    *VideoStateMachine
	q     queue.Queue
	qName string
}

func NewPipelineCoordinator(repo repository.VideoRepository, sm *VideoStateMachine, q queue.Queue, qName string) *PipelineCoordinator {
	return &PipelineCoordinator{
		repo:  repo,
		sm:    sm,
		q:     q,
		qName: qName,
	}
}

// Advance is called by workers when a stage is successfully completed.
func (pc *PipelineCoordinator) Advance(ctx context.Context, id string) error {
	v, err := pc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	currentStage, err := pc.sm.Get(v.Stage)
	if err != nil {
		return err
	}

	// Ask state machine for next stage (respecting rules)
	nextStage, err := pc.sm.NextStage(v.Stage)
	if err != nil {
		return err
	}
	if err := pc.sm.CanTransition(currentStage, nextStage); err != nil {
		return err
	}
	// can transit to phase 2
	if currentStage.Parent != nextStage.Parent && (v.Title == nil) {
		fmt.Println("waiting for user confirmation before transit to pase 2")
		return nil
	}
	// Update DB
	if err := pc.repo.UpdateState(ctx, id, nextStage.Ordering, nextStage.Name); err != nil {
		return err
	}

	// If terminal, do nothing more
	if nextStage.IsTerminal {
		return nil
	}

	// Enqueue next work item
	payload, err := json.Marshal(jobs.JobPayload{
		VideoID: id,
		Stage:   nextStage.Ordering,
		Status:  nextStage.Name,
	})
	if err != nil {
		return err
	}

	return pc.q.Enqueue(ctx, pc.qName, payload)
}

// Fail is called when a worker fails the stage.
func (pc *PipelineCoordinator) Fail(ctx context.Context, id string, reason string) error {
	v, err := pc.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	current, err := pc.sm.Get(v.Stage)
	if err != nil {
		return err
	}

	// Ask state machine for error transition
	errorStage, err := pc.sm.GetErrorStage(current.Parent)
	if err != nil {
		return err
	}

	// Update DB to error stage
	if err := pc.repo.UpdateState(ctx, id, v.Stage, errorStage.Name); err != nil {
		return err
	}

	// error stages are terminal → no queueing
	return nil
}
