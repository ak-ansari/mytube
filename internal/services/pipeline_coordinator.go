package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
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
	// Update DB atomically: only the caller that moves the video off currentStage enqueues the next job
	advanced, err := pc.repo.AdvanceState(ctx, id, currentStage.Ordering, nextStage.Ordering, nextStage.Name)
	if err != nil {
		return err
	}
	if !advanced {
		fmt.Printf("video %s already advanced past stage %d (or failed) by another caller, skipping\n", id, currentStage.Ordering)
		return nil
	}

	return pc.enqueue(ctx, id, nextStage)
}

// enqueue queues the job that runs stage (nothing for a terminal stage).
func (pc *PipelineCoordinator) enqueue(ctx context.Context, id string, stage *models.Stage) error {
	if stage.IsTerminal {
		return nil
	}
	payload, err := json.Marshal(jobs.JobPayload{
		VideoID: id,
		Stage:   stage.Ordering,
		Status:  stage.Name,
	})
	if err != nil {
		return err
	}
	return pc.q.Enqueue(ctx, pc.qName, payload)
}

// AdvanceFrom advances the video only if it is still at fromStage, so a duplicate or stale caller can't
// move it twice. With resume set (a retry after a failed Advance) and the video already one stage
// further, the earlier AdvanceState committed but enqueuing the next job failed: queue that job again.
// Failed videos and anything further along are left alone.
func (pc *PipelineCoordinator) AdvanceFrom(ctx context.Context, id string, fromStage int, resume bool) error {
	v, err := pc.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if isFailed(v.Status) {
		return nil
	}
	switch {
	case v.Stage == fromStage:
		return pc.Advance(ctx, id)
	case resume && v.Stage == fromStage+1:
		current, err := pc.sm.Get(v.Stage)
		if err != nil {
			return err
		}
		return pc.enqueue(ctx, id, current)
	default:
		return nil
	}
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
