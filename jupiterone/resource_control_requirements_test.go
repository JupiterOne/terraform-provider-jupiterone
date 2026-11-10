package jupiterone

import (
	"reflect"
	"testing"
)

func TestOrderRequirementIds(t *testing.T) {
	tests := []struct {
		name   string
		prior  []string
		actual []string
		want   []string
	}{
		{
			name:   "no prior state sorts for a deterministic import",
			prior:  nil,
			actual: []string{"c", "a", "b"},
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "keeps the configured order when nothing changed",
			prior:  []string{"c", "a", "b"},
			actual: []string{"a", "b", "c"},
			want:   []string{"c", "a", "b"},
		},
		{
			name:   "drops ids removed outside terraform",
			prior:  []string{"c", "a", "b"},
			actual: []string{"b", "c"},
			want:   []string{"c", "b"},
		},
		{
			name:   "appends ids linked outside terraform in sorted order",
			prior:  []string{"b"},
			actual: []string{"z", "b", "m"},
			want:   []string{"b", "m", "z"},
		},
		{
			name:   "no links reads as an empty list, not null",
			prior:  []string{"a"},
			actual: nil,
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orderRequirementIds(tt.prior, tt.actual)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("orderRequirementIds(%v, %v) = %v, want %v", tt.prior, tt.actual, got, tt.want)
			}
		})
	}
}
