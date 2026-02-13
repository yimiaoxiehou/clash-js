package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractBandwidthMbps(t *testing.T) {
	tests := []struct {
		line string
		want float64
		ok   bool
	}{
		{"HK-1 带宽:250M", 250, true},
		{"US-1 bandwidth=0.5G", 512, true},
		{"JP-1, 199m", 199, true},
		{"IP:1.1.1.1", 0, false},
	}

	for _, tt := range tests {
		got, ok := extractBandwidthMbps(tt.line)
		if ok != tt.ok {
			t.Fatalf("line=%q ok=%v want=%v", tt.line, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Fatalf("line=%q got=%v want=%v", tt.line, got, tt.want)
		}
	}
}

func TestFilterLines(t *testing.T) {
	input := "\nnode-a 带宽:120M\nnode-b 带宽:250M\nnode-c|0.3g\nnode-d|200m\n"
	got := filterLines(input, 200)
	if len(got) != 2 {
		t.Fatalf("len=%d want=2", len(got))
	}
	if got[0] != "node-b 带宽:250M" || got[1] != "node-c|0.3g" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestFilterLinesWithResultTable(t *testing.T) {
	html := `<div id="result"><table><tbody>
	<tr><td>1</td><td>1.1.1.1</td><td>x</td><td>x</td><td>x</td><td>250M</td></tr>
	<tr><td>2</td><td>2.2.2.2</td><td>x</td><td>x</td><td>x</td><td>150M</td></tr>
	<tr><td>3</td><td>3.3.3.3</td><td>x</td><td>x</td><td>x</td><td>0.5G</td></tr>
	</tbody></table></div>`

	got := filterLines(html, 200)
	if len(got) != 2 {
		t.Fatalf("len=%d want=2", len(got))
	}
	if got[0] != "1.1.1.1 带宽:250M" || got[1] != "3.3.3.3 带宽:0.5G" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestNodesAPI(t *testing.T) {
	store := &NodeStore{}
	now := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	store.Set([]string{"node-a 带宽:250M", "node-b|0.5g"}, now, nil)

	r := newRouter(store, 200)
	req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", w.Code, http.StatusOK)
	}

	var payload struct {
		Count      int      `json:"count"`
		ThresholdM float64  `json:"threshold_m"`
		UpdatedAt  string   `json:"updated_at"`
		LastError  string   `json:"last_error"`
		Nodes      []string `json:"nodes"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if payload.Count != 2 || len(payload.Nodes) != 2 {
		t.Fatalf("unexpected node count: %#v", payload)
	}
	if payload.ThresholdM != 200 {
		t.Fatalf("threshold=%v want=200", payload.ThresholdM)
	}
	if payload.UpdatedAt != now.Format(time.RFC3339) {
		t.Fatalf("updated_at=%q want=%q", payload.UpdatedAt, now.Format(time.RFC3339))
	}
	if payload.LastError != "" {
		t.Fatalf("last_error=%q want empty", payload.LastError)
	}
}
