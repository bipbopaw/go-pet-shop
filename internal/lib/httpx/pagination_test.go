package httpx

import (
	"net/url"
	"testing"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
		wantErr    bool
	}{
		{name: "defaults", wantLimit: 20},
		{name: "custom values", query: "limit=5&offset=10", wantLimit: 5, wantOffset: 10},
		{name: "minimum limit", query: "limit=1", wantLimit: 1},
		{name: "maximum limit", query: "limit=100", wantLimit: 100},
		{name: "zero limit", query: "limit=0", wantErr: true},
		{name: "limit above maximum", query: "limit=101", wantErr: true},
		{name: "invalid limit", query: "limit=abc", wantErr: true},
		{name: "negative offset", query: "offset=-1", wantErr: true},
		{name: "invalid offset", query: "offset=abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tt.query)
			limit, offset, err := ParsePagination(query, 20, 100)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got %v", tt.wantErr, err)
			}
			if err == nil && (limit != tt.wantLimit || offset != tt.wantOffset) {
				t.Fatalf("expected limit=%d offset=%d, got limit=%d offset=%d", tt.wantLimit, tt.wantOffset, limit, offset)
			}
		})
	}
}
