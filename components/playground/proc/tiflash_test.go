package proc

import "testing"

func TestIsKeyPresentInMap(t *testing.T) {
	cases := []struct {
		name string
		m    map[string]any
		key  string
		want bool
	}{
		{name: "empty-map", m: nil, key: "a", want: false},
		{name: "empty-key", m: map[string]any{"a": 1}, key: "", want: false},
		{name: "top-level-present", m: map[string]any{"a": 1}, key: "a", want: true},
		{name: "top-level-missing", m: map[string]any{"a": 1}, key: "b", want: false},
		{name: "nested-present", m: map[string]any{"a": map[string]any{"b": 1}}, key: "a.b", want: true},
		{name: "nested-missing", m: map[string]any{"a": map[string]any{"b": 1}}, key: "a.c", want: false},
		{name: "intermediate-not-map", m: map[string]any{"a": 1, "b": 2}, key: "a.b", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isKeyPresentInMap(tc.m, tc.key); got != tc.want {
				t.Fatalf("isKeyPresentInMap()=%v want %v", got, tc.want)
			}
		})
	}
}
