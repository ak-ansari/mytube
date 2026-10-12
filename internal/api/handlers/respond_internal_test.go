package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func testContext(target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return c, w
}

type envelope struct {
	Message string          `json:"message"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Status  int             `json:"status"`
	Error   json.RawMessage `json:"error"`
}

func decodeEnv(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return e
}

func TestPaging(t *testing.T) {
	tests := []struct {
		query      string
		page, size int
	}{
		{"", 1, 20},
		{"?page=3&size=10", 3, 10},
		{"?size=50", 1, 50},
		{"?size=51", 1, 50},
		{"?size=1000", 1, 50},
		{"?page=0&size=0", 1, 20},
		{"?page=-2&size=-5", 1, 20},
		{"?page=abc&size=xyz", 1, 20},
		{"?page=1000000&size=50", 199, 50}, // (10000-1)/50
		{"?page=1000000&size=20", 499, 20},
		{"?page=9999&size=1", 9999, 1},
		{"?page=10000&size=1", 9999, 1},
	}
	for _, tt := range tests {
		c, _ := testContext("/x" + tt.query)
		p := paging(c)
		if p.Page != tt.page || p.Size != tt.size {
			t.Errorf("paging(%q) = %+v, want page %d size %d", tt.query, p, tt.page, tt.size)
		}
		// services fetch size+1 rows from offset; must stay inside ES's max_result_window
		if p.Offset()+p.Size+1 > maxResultWindow {
			t.Errorf("paging(%q) exceeds the result window", tt.query)
		}
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		msg    string
	}{
		{"not found", services.ErrNotFound, 404, "not found"},
		{"wrapped not found", fmt.Errorf("x: %w", services.ErrNotFound), 404, "not found"},
		{"forbidden", services.ErrForbidden, 403, "you are not allowed to do that"},
		{"bad request message is the detail", fmt.Errorf("%w: title must be 3-100 characters", services.ErrBadRequest), 400, "title must be 3-100 characters"},
		{"bare bad request", services.ErrBadRequest, 400, "bad request"},
		{"unexpected errors are hidden", errors.New("pq: password authentication failed"), 500, "something went wrong"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, w := testContext("/x")
			respondError(c, tt.err)
			env := decodeEnv(t, w)
			if w.Code != tt.status || env.Status != tt.status || env.Message != tt.msg || env.Success {
				t.Errorf("got %d %+v", w.Code, env)
			}
			if string(env.Data) != "null" || string(env.Error) != "null" {
				t.Errorf("data/error should be null, got %s / %s", env.Data, env.Error)
			}
		})
	}
}

func TestRespondOKAndBadRequest(t *testing.T) {
	c, w := testContext("/x")
	respondOK(c, "done", map[string]int{"n": 1})
	env := decodeEnv(t, w)
	if w.Code != 200 || !env.Success || env.Status != 200 || env.Message != "done" || string(env.Data) != `{"n":1}` {
		t.Errorf("respondOK = %d %+v", w.Code, env)
	}

	c, w = testContext("/x")
	badRequest(c, "nope")
	env = decodeEnv(t, w)
	if w.Code != 400 || env.Success || env.Status != 400 || env.Message != "nope" {
		t.Errorf("badRequest = %d %+v", w.Code, env)
	}
}

func TestUUIDParam(t *testing.T) {
	c, w := testContext("/x")
	c.Params = gin.Params{{Key: "id", Value: "65ae26a1-c831-4010-8a2e-82a2e35b0292"}}
	if id, ok := uuidParam(c, "id"); !ok || id.String() != "65ae26a1-c831-4010-8a2e-82a2e35b0292" {
		t.Fatalf("valid uuid rejected")
	}
	if w.Body.Len() != 0 {
		t.Error("valid param must not write a response")
	}
	c, w = testContext("/x")
	c.Params = gin.Params{{Key: "commentId", Value: "nope"}}
	if _, ok := uuidParam(c, "commentId"); ok {
		t.Fatal("invalid uuid accepted")
	}
	if env := decodeEnv(t, w); w.Code != 400 || env.Message != "invalid commentId" {
		t.Errorf("got %d %+v", w.Code, env)
	}
}
