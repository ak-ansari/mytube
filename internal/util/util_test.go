package util_test

import (
	"encoding/json"
	"testing"

	"github.com/ak-ansari/mytube/internal/util"
)

func TestParseIntWithDefault(t *testing.T) {
	tests := []struct {
		in   string
		def  int
		want int
	}{
		{"", 7, 7},
		{"5", 7, 5},
		{"0", 7, 7},
		{"-3", 7, 7},
		{"abc", 7, 7},
		{"1.5", 7, 7},
		{" 5", 7, 7},
		{"99999999999999999999", 7, 7},
	}
	for _, tt := range tests {
		if got := util.ParseIntWithDefault(tt.in, tt.def); got != tt.want {
			t.Errorf("ParseIntWithDefault(%q, %d) = %d, want %d", tt.in, tt.def, got, tt.want)
		}
	}
}

func TestQualities(t *testing.T) {
	m := util.GetQualityMap()
	if len(m) != len(util.Sizes) {
		t.Fatalf("map has %d entries", len(m))
	}
	for i, q := range util.Sizes {
		if m[q.Label] != q {
			t.Errorf("map[%s] = %+v", q.Label, m[q.Label])
		}
		if i > 0 && (q.Height <= util.Sizes[i-1].Height || q.Bandwidth <= util.Sizes[i-1].Bandwidth) {
			t.Errorf("Sizes must be ascending, %s after %s", q.Label, util.Sizes[i-1].Label)
		}
	}
	if m["720p"].Width != 1280 || m["720p"].Height != 720 {
		t.Errorf("720p = %+v", m["720p"])
	}
}

func TestNewResponse(t *testing.T) {
	tests := []struct {
		status  int
		success bool
	}{
		{200, true}, {201, true}, {204, true}, {302, true}, {399, true},
		{400, false}, {401, false}, {404, false}, {500, false}, {199, false},
	}
	for _, tt := range tests {
		r := util.NewResponse(tt.status, "m", nil, nil)
		if r.Success != tt.success || r.Status != tt.status {
			t.Errorf("NewResponse(%d) = %+v", tt.status, r)
		}
	}
	b, _ := json.Marshal(util.NewResponse(200, "ok", []int{1}, nil))
	if string(b) != `{"message":"ok","success":true,"data":[1],"status":200,"error":null}` {
		t.Errorf("json = %s", b)
	}
}
