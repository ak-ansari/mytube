package services

import (
	"errors"
	"fmt"

	"github.com/ak-ansari/mytube/internal/models"
)

var (
	ErrorInvalidStage = errors.New("invalid stage order")
)

type VideoStateMachine struct {
}

func NewVideoStateMachine() *VideoStateMachine {
	return &VideoStateMachine{}
}

func (sm *VideoStateMachine) NextStage(current int) (*models.Stage, error) {
	return sm.Get(current + 1)
}
func (sm *VideoStateMachine) GetDefault() (*models.Stage, error) {
	return sm.Get(1)
}
func (sm *VideoStateMachine) Get(order int) (*models.Stage, error) {
	stage, exists := models.Stages[order]
	if !exists {
		return nil, fmt.Errorf("%w, stage: %d", ErrorInvalidStage, order)
	}
	return stage, nil
}

func (sm *VideoStateMachine) GetByName(name models.VideoStatus) (*models.Stage, error) {
	for _, stage := range models.Stages {
		if stage.Name == name {
			return stage, nil
		}
	}
	return nil, fmt.Errorf("%w, name: %s", ErrorInvalidStage, name)
}
func (sm *VideoStateMachine) CanTransition(current, next *models.Stage) error {
	if current.IsTerminal {
		return fmt.Errorf("can not transit from a terminal stage")
	}
	// check for ordering
	if current.Ordering+1 != next.Ordering {
		return fmt.Errorf("invalid transition")
	}
	return nil
}
func (sm *VideoStateMachine) GetErrorStage(phase models.ParentStage) (*models.Stage, error) {
	stage, ok := models.ErrorStages[phase]
	if !ok {
		return nil, fmt.Errorf("no stage was found with parent:%s", phase)
	}
	return stage, nil
}
