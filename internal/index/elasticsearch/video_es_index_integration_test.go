//go:build integration

package elasticsearch_index

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/index"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/testutil/integration"
	es_v9 "github.com/elastic/go-elasticsearch/v9"
	"github.com/google/uuid"
)

var _ index.VideoIndex = (*VideoEsIndex)(nil)

// newTestIndex creates a throwaway index with the production mapping and returns a
// VideoEsIndex bound to it (the real constructor hardcodes video_index). It is deleted afterwards.
func newTestIndex(t *testing.T) (*VideoEsIndex, *es_v9.Client) {
	t.Helper()
	c := integration.Elastic(t)
	name := "it_video_index_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	body, _ := json.Marshal(getCreateVideoIndexCnf())
	res, err := c.Indices.Create(name, c.Indices.Create.WithBody(bytes.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.IsError() {
		t.Fatalf("create index: %s", res.String())
	}
	t.Cleanup(func() {
		if r, err := c.Indices.Delete([]string{name}); err == nil {
			r.Body.Close()
		}
	})
	return &VideoEsIndex{client: c, index: name}, c
}

func refresh(t *testing.T, vi *VideoEsIndex) {
	t.Helper()
	res, err := vi.client.Indices.Refresh(vi.client.Indices.Refresh.WithIndex(vi.index))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
}

func video(title, desc string) *models.Video {
	return &models.Video{ID: uuid.New(), Title: &title, Description: &desc}
}

func contains(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// search polls briefly on top of an explicit refresh to absorb near-real-time lag.
func search(t *testing.T, vi *VideoEsIndex, q string, cond func([]string) bool) []string {
	t.Helper()
	var ids []string
	integration.Eventually(t, 5*time.Second, func() bool {
		refresh(t, vi)
		var err error
		ids, err = vi.SearchVideo(context.Background(), q, []string{"title", "description"}, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		return cond(ids)
	}, "search %q never matched, last = %v", q, ids)
	return ids
}

func TestNewVideoEsIndex_UsesVideoIndex(t *testing.T) {
	c := integration.Elastic(t)
	res, err := c.Indices.Exists([]string{string(models.IndexVideo)})
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		t.Skip("dev video_index does not exist; not creating it from tests")
	}
	vi, err := NewVideoEsIndex(c) // existing index: read-only path
	if err != nil {
		t.Fatal(err)
	}
	if vi.index != "video_index" {
		t.Fatalf("index = %q", vi.index)
	}
}

func TestVideoEsIndex_InsertAndSearch(t *testing.T) {
	vi, _ := newTestIndex(t)
	ctx := context.Background()
	cats := video("Funny cats compilation", "kittens doing silly things")
	cars := video("Electric trucks review", "range and charging test")
	for _, v := range []*models.Video{cats, cars} {
		if err := vi.InsertVideo(ctx, v); err != nil {
			t.Fatal(err)
		}
	}

	ids := search(t, vi, "cats", func(ids []string) bool { return len(ids) > 0 })
	if len(ids) != 1 || ids[0] != cats.ID.String() {
		t.Fatalf("search cats = %v", ids)
	}
	// description field and fuzziness (typo)
	search(t, vi, "chargng", func(ids []string) bool { return len(ids) == 1 && ids[0] == cars.ID.String() })

	none, err := vi.SearchVideo(ctx, "zzzzqqqq", []string{"title", "description"}, 0, 10)
	if err != nil || len(none) != 0 {
		t.Fatalf("no-match search = %v, %v", none, err)
	}
	// field restriction
	onlyTitle, _ := vi.SearchVideo(ctx, "kittens", []string{"title"}, 0, 10)
	if len(onlyTitle) != 0 {
		t.Fatalf("title-only search matched description: %v", onlyTitle)
	}
}

func TestVideoEsIndex_SearchPaging(t *testing.T) {
	vi, _ := newTestIndex(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := vi.InsertVideo(ctx, video("guitar lesson", "part")); err != nil {
			t.Fatal(err)
		}
	}
	search(t, vi, "guitar", func(ids []string) bool { return len(ids) == 5 })
	p1, _ := vi.SearchVideo(ctx, "guitar", []string{"title"}, 0, 3)
	p2, _ := vi.SearchVideo(ctx, "guitar", []string{"title"}, 3, 3)
	if len(p1) != 3 || len(p2) != 2 {
		t.Fatalf("pages = %d + %d", len(p1), len(p2))
	}
	for _, id := range p2 {
		if contains(p1, id) {
			t.Fatalf("id %s on both pages", id)
		}
	}
}

func TestVideoEsIndex_UpdateReplacesDocument(t *testing.T) {
	vi, _ := newTestIndex(t)
	ctx := context.Background()
	v := video("Old title about pianos", "first")
	if err := vi.InsertVideo(ctx, v); err != nil {
		t.Fatal(err)
	}
	search(t, vi, "pianos", func(ids []string) bool { return contains(ids, v.ID.String()) })

	newTitle, newDesc := "Brand new drums video", "second"
	v.Title, v.Description = &newTitle, &newDesc
	if err := vi.UpdateVideo(ctx, v); err != nil {
		t.Fatal(err)
	}
	search(t, vi, "drums", func(ids []string) bool { return len(ids) == 1 && ids[0] == v.ID.String() })
	if ids, _ := vi.SearchVideo(ctx, "pianos", []string{"title", "description"}, 0, 10); len(ids) != 0 {
		t.Fatalf("old title still indexed: %v", ids)
	}
	var count struct {
		Count int `json:"count"`
	}
	res, err := vi.client.Count(vi.client.Count.WithIndex(vi.index))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	_ = json.NewDecoder(res.Body).Decode(&count)
	if count.Count != 1 {
		t.Fatalf("doc count = %d, want 1 (update must not duplicate)", count.Count)
	}
}

func TestVideoEsIndex_Suggest(t *testing.T) {
	vi, _ := newTestIndex(t)
	ctx := context.Background()
	a := video("Learning golang basics", "tutorial")
	b := video("Lemon cake recipe", "baking")
	for _, v := range []*models.Video{a, b} {
		if err := vi.InsertVideo(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	refresh(t, vi)

	var sugg []index.Suggestion
	integration.Eventually(t, 5*time.Second, func() bool {
		var err error
		sugg, err = vi.SuggestVideos(ctx, "lear")
		if err != nil {
			t.Fatal(err)
		}
		return len(sugg) > 0
	}, "no suggestions")
	if len(sugg) != 1 || sugg[0].ID != a.ID.String() || sugg[0].Text != "Learning golang basics tutorial" {
		t.Fatalf("suggest lear = %+v", sugg)
	}
	// fuzzy prefix (1 edit)
	fuzzy, _ := vi.SuggestVideos(ctx, "lemn")
	found := false
	for _, s := range fuzzy {
		found = found || s.ID == b.ID.String()
	}
	if !found {
		t.Fatalf("fuzzy suggest lemn = %+v", fuzzy)
	}
	none, err := vi.SuggestVideos(ctx, "xyzxyz")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("no-match suggest = %#v, %v", none, err)
	}
}

func TestVideoEsIndex_Delete(t *testing.T) {
	vi, _ := newTestIndex(t)
	ctx := context.Background()
	v := video("Deleted violin video", "x")
	keep := video("Kept violin video", "y")
	_ = vi.InsertVideo(ctx, v)
	_ = vi.InsertVideo(ctx, keep)
	search(t, vi, "violin", func(ids []string) bool { return len(ids) == 2 })

	if err := vi.DeleteVideo(ctx, v.ID.String()); err != nil {
		t.Fatal(err)
	}
	search(t, vi, "violin", func(ids []string) bool { return len(ids) == 1 && ids[0] == keep.ID.String() })
	// never-indexed / already deleted -> 404 is swallowed
	if err := vi.DeleteVideo(ctx, v.ID.String()); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if err := vi.DeleteVideo(ctx, uuid.NewString()); err != nil {
		t.Fatalf("delete unknown: %v", err)
	}
}

func TestVideoEsIndex_ErrorsOnMissingIndex(t *testing.T) {
	c := integration.Elastic(t)
	vi := &VideoEsIndex{client: c, index: "it_missing_" + strings.ReplaceAll(uuid.NewString(), "-", "")}
	if _, err := vi.SearchVideo(context.Background(), "x", []string{"title"}, 0, 10); err == nil {
		t.Fatal("search on missing index should error")
	}
	if _, err := vi.SuggestVideos(context.Background(), "x"); err == nil {
		t.Fatal("suggest on missing index should error")
	}
}
