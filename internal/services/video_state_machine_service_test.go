package services_test

import (
	"errors"
	"testing"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
)

func TestStateMachineGet(t *testing.T) {
	sm := services.NewVideoStateMachine()
	tests := []struct {
		order   int
		want    models.VideoStatus
		wantErr bool
	}{
		{0, "", true},
		{1, models.StatusUploadPending, false},
		{2, models.StatusUploaded, false},
		{3, models.StatusValid, false},
		{4, models.StatusThumbnailGenerated, false},
		{5, models.StatusTranscoded, false},
		{6, models.StatusSegmentGenerated, false},
		{7, models.StatusChecksum, false},
		{8, models.StatusPublished, false},
		{9, "", true},
		{-1, "", true},
	}
	for _, tt := range tests {
		s, err := sm.Get(tt.order)
		if tt.wantErr {
			if !errors.Is(err, services.ErrorInvalidStage) {
				t.Errorf("Get(%d) err = %v, want ErrorInvalidStage", tt.order, err)
			}
			continue
		}
		if err != nil || s.Name != tt.want || s.Ordering != tt.order {
			t.Errorf("Get(%d) = %+v, %v", tt.order, s, err)
		}
	}

	def, err := sm.GetDefault()
	if err != nil || def.Ordering != 1 || def.Name != models.StatusUploadPending {
		t.Errorf("GetDefault = %+v, %v", def, err)
	}
}

func TestStateMachineNextStage(t *testing.T) {
	sm := services.NewVideoStateMachine()
	for cur := 1; cur <= 7; cur++ {
		next, err := sm.NextStage(cur)
		if err != nil || next.Ordering != cur+1 {
			t.Errorf("NextStage(%d) = %+v, %v", cur, next, err)
		}
	}
	if _, err := sm.NextStage(8); !errors.Is(err, services.ErrorInvalidStage) {
		t.Errorf("NextStage(8) err = %v", err)
	}
}

func TestStateMachineCanTransition(t *testing.T) {
	sm := services.NewVideoStateMachine()
	get := func(o int) *models.Stage { s, _ := sm.Get(o); return s }
	tests := []struct {
		name     string
		cur, nxt *models.Stage
		ok       bool
	}{
		{"1->2", get(1), get(2), true},
		{"3->4 across phases", get(3), get(4), true},
		{"7->8", get(7), get(8), true},
		{"skip 1->3", get(1), get(3), false},
		{"backwards 4->3", get(4), get(3), false},
		{"same 2->2", get(2), get(2), false},
		{"from terminal", get(8), &models.Stage{Ordering: 9}, false},
		{"from error stage", models.ErrorStages[models.ParentStagePhase1], get(2), false},
	}
	for _, tt := range tests {
		if err := sm.CanTransition(tt.cur, tt.nxt); (err == nil) != tt.ok {
			t.Errorf("%s: err = %v", tt.name, err)
		}
	}
}

func TestStateMachineErrorStage(t *testing.T) {
	sm := services.NewVideoStateMachine()
	tests := []struct {
		phase   models.ParentStage
		want    models.VideoStatus
		wantErr bool
	}{
		{models.ParentStagePhase1, models.StatusPhase1Failed, false},
		{models.ParentStagePhase2, models.StatusPhase2Failed, false},
		{"phase_3", "", true},
	}
	for _, tt := range tests {
		s, err := sm.GetErrorStage(tt.phase)
		if (err != nil) != tt.wantErr {
			t.Errorf("GetErrorStage(%s) err = %v", tt.phase, err)
			continue
		}
		if !tt.wantErr && (s.Name != tt.want || !s.IsTerminal) {
			t.Errorf("GetErrorStage(%s) = %+v", tt.phase, s)
		}
	}
}

func TestStateMachineGetByName(t *testing.T) {
	sm := services.NewVideoStateMachine()
	tests := []struct {
		name models.VideoStatus
		want int
	}{
		{models.StatusUploadPending, 1},
		{models.StatusUploaded, 2},
		{models.StatusValid, 3},
		{models.StatusThumbnailGenerated, 4},
		{models.StatusTranscoded, 5},
		{models.StatusSegmentGenerated, 6},
		{models.StatusChecksum, 7},
		{models.StatusPublished, 8},
	}
	for _, tt := range tests {
		s, err := sm.GetByName(tt.name)
		if err != nil || s.Ordering != tt.want {
			t.Errorf("GetByName(%s) = %+v, %v", tt.name, s, err)
		}
	}
	for _, unknown := range []models.VideoStatus{models.StatusDone, models.StatusPhase1Failed, "nope"} {
		if _, err := sm.GetByName(unknown); !errors.Is(err, services.ErrorInvalidStage) {
			t.Errorf("GetByName(%s) err = %v", unknown, err)
		}
	}
}

// Stage names are unique, so GetByName (which ranges over a map) is deterministic; "published" is the terminal stage 8.
func TestStateMachineGetByNamePublishedIsDeterministic(t *testing.T) {
	names := map[models.VideoStatus]int{}
	for order, st := range models.Stages {
		if prev, dup := names[st.Name]; dup {
			t.Errorf("stages %d and %d share the name %q", prev, order, st.Name)
		}
		names[st.Name] = order
	}
	sm := services.NewVideoStateMachine()
	for i := 0; i < 500; i++ {
		s, err := sm.GetByName(models.StatusPublished)
		if err != nil {
			t.Fatal(err)
		}
		if s.Ordering != 8 || !s.IsTerminal {
			t.Fatalf("GetByName(published) = %+v, want terminal stage 8", s)
		}
	}
}
