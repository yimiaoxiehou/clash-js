package main

import "testing"

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
