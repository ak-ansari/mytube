package models_test

import (
	"encoding/json"
	"testing"

	"github.com/ak-ansari/mytube/internal/models"
)

func TestNewPage(t *testing.T) {
	tests := []struct {
		name    string
		items   []int
		size    int
		want    int
		hasMore bool
	}{
		{"nil items -> empty slice", nil, 10, 0, false},
		{"fewer than size", []int{1, 2}, 10, 2, false},
		{"exactly size", []int{1, 2, 3}, 3, 3, false},
		{"size+1 trims and has_more", []int{1, 2, 3, 4}, 3, 3, true},
		{"many more", []int{1, 2, 3, 4, 5, 6}, 2, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := models.NewPage(tt.items, 4, tt.size)
			if p.Items == nil || len(p.Items) != tt.want || p.HasMore != tt.hasMore || p.Page != 4 || p.Size != tt.size {
				t.Fatalf("page = %+v", p)
			}
			for i, v := range p.Items {
				if v != tt.items[i] {
					t.Fatalf("order changed: %v", p.Items)
				}
			}
		})
	}
	b, _ := json.Marshal(models.NewPage[int](nil, 1, 20))
	if string(b) != `{"items":[],"page":1,"size":20,"has_more":false}` {
		t.Errorf("json = %s", b)
	}
}

func TestStagesTable(t *testing.T) {
	if len(models.Stages) != 8 {
		t.Fatalf("expected 8 stages, got %d", len(models.Stages))
	}
	terminal := 0
	for key, s := range models.Stages {
		if s.Ordering != key {
			t.Errorf("stage key %d has Ordering %d", key, s.Ordering)
		}
		wantParent := models.ParentStagePhase2
		if key <= 3 {
			wantParent = models.ParentStagePhase1
		}
		if s.Parent != wantParent {
			t.Errorf("stage %d parent = %s, want %s", key, s.Parent, wantParent)
		}
		if s.IsTerminal {
			terminal++
			if key != 8 {
				t.Errorf("only the last stage should be terminal, got %d", key)
			}
		}
	}
	if terminal != 1 {
		t.Errorf("%d terminal stages", terminal)
	}
	// each successive stage is exactly one step further: the coordinator relies on Ordering+1
	for i := 1; i < 8; i++ {
		if models.Stages[i+1].Ordering != models.Stages[i].Ordering+1 {
			t.Errorf("gap between stage %d and %d", i, i+1)
		}
	}
	for phase, s := range models.ErrorStages {
		if !s.IsTerminal || s.Parent != phase {
			t.Errorf("error stage %s = %+v", phase, s)
		}
		if _, clash := models.Stages[s.Ordering]; clash {
			t.Errorf("error stage ordering %d clashes with a pipeline stage", s.Ordering)
		}
	}
	if models.ErrorStages[models.ParentStagePhase1].Name != models.StatusPhase1Failed ||
		models.ErrorStages[models.ParentStagePhase2].Name != models.StatusPhase2Failed {
		t.Error("error stage names")
	}
}

func TestEmbeddedJSONShapes(t *testing.T) {
	d := models.VideoDetails{VideoCard: models.VideoCard{Status: models.StatusPublished}, Likes: 3}
	b, _ := json.Marshal(d)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"id", "status", "likes", "manifest_url", "channel", "thumbnail_url", "my_reaction"} {
		if _, ok := m[k]; !ok {
			t.Errorf("video details JSON missing %q: %s", k, b)
		}
	}
	p := models.Profile{Channel: models.Channel{ChannelSummary: models.ChannelSummary{UserName: "u"}}}
	b, _ = json.Marshal(p)
	m = nil
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"id", "user_name", "avatar_url", "subscribers", "is_subscribed", "email"} {
		if _, ok := m[k]; !ok {
			t.Errorf("profile JSON missing %q: %s", k, b)
		}
	}
}
