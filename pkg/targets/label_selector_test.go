package targets

import (
	"reflect"
	"testing"
)

func TestParseLabelSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		want     Labels
	}{
		{
			name:     "empty selector",
			selector: "",
			want: Labels{
				Included: []string{},
				Excluded: []string{},
			},
		},
		{
			name:     "single included label",
			selector: "konveyor.io/target=cloud-readiness",
			want: Labels{
				Included: []string{"konveyor.io/target=cloud-readiness"},
				Excluded: []string{},
			},
		},
		{
			name:     "multiple included labels with OR",
			selector: "konveyor.io/target=cloud-readiness || konveyor.io/target=linux",
			want: Labels{
				Included: []string{"konveyor.io/target=cloud-readiness", "konveyor.io/target=linux"},
				Excluded: []string{},
			},
		},
		{
			name:     "single excluded label",
			selector: "!konveyor.io/target=windows",
			want: Labels{
				Included: []string{},
				Excluded: []string{"konveyor.io/target=windows"},
			},
		},
		{
			name:     "mixed included and excluded",
			selector: "konveyor.io/target=quarkus || !konveyor.io/source=java8",
			want: Labels{
				Included: []string{"konveyor.io/target=quarkus"},
				Excluded: []string{"konveyor.io/source=java8"},
			},
		},
		{
			name:     "multiple excluded labels",
			selector: "!konveyor.io/target=windows || !konveyor.io/target=macos",
			want: Labels{
				Included: []string{},
				Excluded: []string{"konveyor.io/target=windows", "konveyor.io/target=macos"},
			},
		},
		{
			name:     "complex mix",
			selector: "konveyor.io/target=cloud-readiness || konveyor.io/target=linux || !konveyor.io/source=java8 || !konveyor.io/source=java11",
			want: Labels{
				Included: []string{"konveyor.io/target=cloud-readiness", "konveyor.io/target=linux"},
				Excluded: []string{"konveyor.io/source=java8", "konveyor.io/source=java11"},
			},
		},
		{
			name:     "selector with extra whitespace",
			selector: "  konveyor.io/target=quarkus  ||  konveyor.io/target=springboot  ",
			want: Labels{
				Included: []string{"konveyor.io/target=quarkus", "konveyor.io/target=springboot"},
				Excluded: []string{},
			},
		},
		{
			name:     "selector with whitespace around exclusion",
			selector: "  !  konveyor.io/target=windows  ",
			want: Labels{
				Included: []string{},
				Excluded: []string{"konveyor.io/target=windows"},
			},
		},
		{
			name:     "real world example from tackle-testapp",
			selector: "konveyor.io/target=cloud-readiness || konveyor.io/target=linux",
			want: Labels{
				Included: []string{"konveyor.io/target=cloud-readiness", "konveyor.io/target=linux"},
				Excluded: []string{},
			},
		},
		{
			name:     "key-only label (existence check)",
			selector: "konveyor.io/target",
			want: Labels{
				Included: []string{"konveyor.io/target"},
				Excluded: []string{},
			},
		},
		{
			name:     "key-only label with exclusion",
			selector: "!konveyor.io/source",
			want: Labels{
				Included: []string{},
				Excluded: []string{"konveyor.io/source"},
			},
		},
		{
			name:     "key-only mixed with key-value",
			selector: "konveyor.io/target || konveyor.io/source=java",
			want: Labels{
				Included: []string{"konveyor.io/target", "konveyor.io/source=java"},
				Excluded: []string{},
			},
		},
		{
			name:     "simple AND operation",
			selector: "konveyor.io/target=quarkus && konveyor.io/source=java",
			want: Labels{
				Included: []string{"konveyor.io/target=quarkus", "konveyor.io/source=java"},
				Excluded: []string{},
			},
		},
		{
			name:     "AND with parentheses",
			selector: "(konveyor.io/target=quarkus && konveyor.io/source=java)",
			want: Labels{
				Included: []string{"konveyor.io/target=quarkus", "konveyor.io/source=java"},
				Excluded: []string{},
			},
		},
		{
			name:     "AND with exclusion",
			selector: "konveyor.io/target=quarkus && !konveyor.io/source=java8",
			want: Labels{
				Included: []string{"konveyor.io/target=quarkus"},
				Excluded: []string{"konveyor.io/source=java8"},
			},
		},
		{
			name:     "complex expression with exclusions",
			selector: "((konveyor.io/target=quarkus || konveyor.io/target=cloud) && (!konveyor.io/source=java8 || !konveyor.io/source=java11))",
			want: Labels{
				Included: []string{"konveyor.io/target=quarkus", "konveyor.io/target=cloud"},
				Excluded: []string{"konveyor.io/source=java8", "konveyor.io/source=java11"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLabelSelector(tt.selector)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseLabelSelector() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseLabelSelectorIncludedCount tests that the correct number of included labels are parsed
func TestParseLabelSelectorIncludedCount(t *testing.T) {
	tests := []struct {
		name         string
		selector     string
		wantIncluded int
		wantExcluded int
	}{
		{"no labels", "", 0, 0},
		{"one included", "label=value", 1, 0},
		{"two included", "label1=value1 || label2=value2", 2, 0},
		{"one excluded", "!label=value", 0, 1},
		{"two excluded", "!label1=value1 || !label2=value2", 0, 2},
		{"mixed", "label1=value1 || !label2=value2 || label3=value3", 2, 1},
		{"key-only included", "konveyor.io/target", 1, 0},
		{"key-only excluded", "!konveyor.io/source", 0, 1},
		{"key-only mixed with key-value", "konveyor.io/target || label=value", 2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLabelSelector(tt.selector)
			if len(got.Included) != tt.wantIncluded {
				t.Errorf("ParseLabelSelector() included count = %d, want %d", len(got.Included), tt.wantIncluded)
			}
			if len(got.Excluded) != tt.wantExcluded {
				t.Errorf("ParseLabelSelector() excluded count = %d, want %d", len(got.Excluded), tt.wantExcluded)
			}
		})
	}
}
