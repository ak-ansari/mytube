package models

// Stage corresponds to the "stages" table:
// Data-driven workflow state machine.
type ParentStage string

const (
	ParentStagePhase1 ParentStage = "phase_1"
	ParentStagePhase2 ParentStage = "phase_2"
)

type Stage struct {
	Name       VideoStatus
	Ordering   int
	IsTerminal bool
	Parent     ParentStage
}

// Initialize stages mav
var Stages = map[int]*Stage{
	1: {
		Ordering:   1,
		Name:       StatusUploadPending,
		IsTerminal: false,
		Parent:     ParentStagePhase1,
	},
	2: {
		Ordering:   2,
		Name:       StatusUploaded,
		IsTerminal: false,
		Parent:     ParentStagePhase1,
	},
	3: {
		Ordering:   3,
		Name:       StatusValid,
		IsTerminal: false,
		Parent:     ParentStagePhase1,
	},
	4: {
		Ordering:   4,
		Name:       StatusThumbnailGenerated,
		IsTerminal: false,
		Parent:     ParentStagePhase2,
	},
	5: {
		Ordering:   5,
		Name:       StatusTranscoded,
		IsTerminal: false,
		Parent:     ParentStagePhase2,
	},
	6: {
		Ordering:   6,
		Name:       StatusSegmentGenerated,
		IsTerminal: false,
		Parent:     ParentStagePhase2,
	},
	7: {
		Ordering:   7,
		Name:       StatusPublished,
		IsTerminal: false,
		Parent:     ParentStagePhase2,
	},
	8: {
		Ordering:   8,
		Name:       StatusPublished,
		IsTerminal: false,
		Parent:     ParentStagePhase2,
	},
	9: {
		Ordering:   9,
		Name:       StatusDone,
		IsTerminal: true,
		Parent:     ParentStagePhase2,
	},
}
var ErrorStages = map[ParentStage]*Stage{
	ParentStagePhase1: {
		Ordering:   98,
		Name:       StatusPhase1Failed,
		IsTerminal: true,
		Parent:     ParentStagePhase1,
	},
	ParentStagePhase2: {
		Ordering:   99,
		Name:       StatusPhase2Failed,
		IsTerminal: true,
		Parent:     ParentStagePhase2,
	},
}
