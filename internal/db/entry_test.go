package db

import (
	"testing"
)

func TestNewEntry(t *testing.T) {
	cases := map[string]struct {
		id             uint16
		l              map[string]string
		expectedLabels string
		expectedString string
	}{
		"NewEntry": {
			id:             1111,
			l:              map[string]string{"a": "b"},
			expectedLabels: "a=b",
			expectedString: "1111 a=b",
		},
		"NewEntryEmptyLabel": {
			id:             1111,
			l:              nil,
			expectedLabels: "",
			expectedString: "1111 ",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := NewEntry(tc.id, tc.l)
			if e.ID() != tc.id {
				t.Errorf("TestNewEntry: -want %d, +got: %d\n", e.ID(), tc.id)
			}
			if e.Labels().String() != tc.expectedLabels {
				t.Errorf("TestNewEntry: -want %s, +got: %s\n", e.Labels(), tc.expectedLabels)
			}
			if e.String() != tc.expectedString {
				t.Errorf("TestNewEntry: -want %s, +got: %s\n", e.String(), tc.expectedString)
			}
		})
	}
}
