package db

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func TestNewDB(t *testing.T) {
	cases := map[string]struct {
		initEntries      Entries[uint16]
		setValidation    ValidationFn[uint16]
		deleteValidation ValidationFn[uint16]
		expectedEntries  int
	}{
		"NewWithoutInitEntries": {
			initEntries:     Entries[uint16]{},
			expectedEntries: 0,
		},
		"NewWithInitEntries": {
			initEntries: Entries[uint16]{
				NewEntry(uint16(0), map[string]string{"x": "a1", "y": "z"}),
				NewEntry(uint16(1), map[string]string{"x": "a2", "y": "z"}),
				NewEntry(uint16(4095), map[string]string{"x": "a3"}),
			},
			setValidation: func(id uint16) error {
				return nil
			},
			deleteValidation: func(id uint16) error {
				return nil
			},
			expectedEntries: 3,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries:      tc.initEntries,
				SetValidation:    tc.setValidation,
				DeleteValidation: tc.deleteValidation,
			})
			if d.Count() != int(tc.expectedEntries) {
				t.Errorf("TestNewDB: -want %d, +got: %d\n", len(d.GetAll()), tc.expectedEntries)
			}
		})
	}
}

func TestSet(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"x": "a1", "y": "z"}),
		NewEntry(uint16(1), map[string]string{"x": "a2", "y": "z"}),
		NewEntry(uint16(4095), map[string]string{"x": "a3"}),
	}

	cases := map[string]struct {
		entry                 Entry[uint16]
		setValidation         ValidationFn[uint16]
		errValidationExpected bool
		expectedEntries       int
		hasInitEntryId        uint16
		hasEntryId            uint16
		hasNotEntryId         uint16
	}{
		"SetValidationNil": {
			entry:                 NewEntry(uint16(3000), map[string]string{"aaa": "3000", "bbb": "3000"}),
			errValidationExpected: false,
			expectedEntries:       4,
			hasInitEntryId:        1,
			hasEntryId:            3000,
			hasNotEntryId:         4000,
		},
		"SetValidationSuccess": {
			entry: NewEntry(uint16(3000), map[string]string{"aaa": "3000", "bbb": "3000"}),
			setValidation: func(id uint16) error {
				return nil
			},
			errValidationExpected: false,
			expectedEntries:       4,
			hasInitEntryId:        1,
			hasEntryId:            3000,
			hasNotEntryId:         4000,
		},
		"SetValidationFail": {
			entry: NewEntry(uint16(3000), map[string]string{"aaa": "3000", "bbb": "3000"}),
			setValidation: func(id uint16) error {
				return fmt.Errorf("error")
			},
			errValidationExpected: true,
			expectedEntries:       3,
			hasInitEntryId:        1,
			hasEntryId:            0,
			hasNotEntryId:         3000,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries:   initEntries,
				SetValidation: tc.setValidation,
				DeleteValidation: func(id uint16) error {
					return nil
				},
			})
			err := d.Set(tc.entry)
			if tc.errValidationExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if d.Count() != int(tc.expectedEntries) {
				t.Errorf("TestSet: -want %d, +got: %d\n", tc.expectedEntries, len(d.GetAll()))
			}
			// has init
			if !d.Has(tc.hasInitEntryId) {
				t.Errorf("TestSet: expected entries %d\n", tc.hasInitEntryId)
			}
			// has
			if !d.Has(tc.hasEntryId) {
				t.Errorf("TestSet: expected entries %d\n", tc.hasEntryId)
			}
			// has method - unexpected value
			if d.Has(tc.hasNotEntryId) {
				t.Errorf("TestSet: unexpected entries, but got %d\n", tc.hasNotEntryId)
			}
		})
	}
}

func TestGet(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"x": "a1", "y": "z"}),
		NewEntry(uint16(1), map[string]string{"x": "a2", "y": "z"}),
		NewEntry(uint16(1000), map[string]string{}),
		NewEntry(uint16(4095), map[string]string{"x": "a3"}),
	}

	cases := map[string]struct {
		entryId     uint16
		errExpected bool
	}{
		"Get": {
			entryId:     1000,
			errExpected: false,
		},
		"GetNotFound": {
			entryId:     1111,
			errExpected: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries: initEntries,
			})
			if !tc.errExpected {
				d.Set(NewEntry(tc.entryId, map[string]string{}))
			}
			e, err := d.Get(tc.entryId)
			if tc.errExpected {
				assert.Error(t, err)
				if e != nil {
					t.Errorf("TestGet: unexpected result, got %v, want: nil\n", e)
				}
			} else {
				assert.NoError(t, err)
				if e != nil {

				} else {
					t.Errorf("TestGet: unexpected result, got nil, want: %v\n", e)
				}
			}
		})
	}
}

func TestGetByLabel(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(1), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(4095), map[string]string{"a": "b"}),
	}

	cases := map[string]struct {
		selector map[string]string
		expected int
	}{
		"GetByLabel": {
			selector: map[string]string{
				"a": "b",
			},
			expected: 3,
		},
		"GetByLabelNotFound": {
			selector: map[string]string{
				"1": "2",
			},
			expected: 0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries: initEntries,
			})

			fullselector := labels.NewSelector()
			for k, v := range tc.selector {
				req, err := labels.NewRequirement(k, selection.In, []string{v})
				if err != nil {
					t.Errorf("TestGetByLabel: unexpected error %s\n", err.Error())
				}
				fullselector = fullselector.Add(*req)
			}

			entries := d.GetByLabel(fullselector)
			if len(entries) != tc.expected {
				t.Errorf("TestGetByLabel: unexpected result, got %d, want: %d\n", len(entries), tc.expected)
			}
		})
	}
}

func TestGetAll(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(1), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(4095), map[string]string{"a": "b"}),
	}

	cases := map[string]struct {
		expectedEntries []uint16
	}{
		"GetAll": {
			expectedEntries: []uint16{0, 1, 4095},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries: initEntries,
			})

			entries := d.GetAll()
			gotEntries := make([]uint16, 0, len(entries))
			for _, e := range entries {
				gotEntries = append(gotEntries, e.ID())
			}
			if len(entries) != len(tc.expectedEntries) {
				t.Errorf("TestGetAll: unexpected result, got %d, want: %d\n", len(gotEntries), len(tc.expectedEntries))
			}
			if diff := cmp.Diff(tc.expectedEntries, gotEntries); diff != "" {
				t.Errorf("TestGetAll: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestHas(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(1), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(4095), map[string]string{"a": "b"}),
	}

	cases := map[string]struct {
		entryID  uint16
		expected bool
	}{
		"Has": {
			entryID:  1,
			expected: true,
		},
		"HasNot": {
			entryID:  1111,
			expected: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries: initEntries,
			})

			got := d.Has(tc.entryID)
			if got != tc.expected {
				t.Errorf("TestHas: unexpected result, got %t, want: %t\n", got, tc.expected)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(1), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(4095), map[string]string{"a": "b"}),
	}

	cases := map[string]struct {
		entryID           uint16
		checkID           uint16
		errGetExpected    bool
		errDeleteExpected bool
		deleteValidation  ValidationFn[uint16]
	}{
		"Delete": {
			entryID:           1,
			checkID:           1,
			errGetExpected:    false,
			errDeleteExpected: false,
		},
		"DeleteEmpty": {
			entryID:           1111,
			checkID:           1111,
			errGetExpected:    false,
			errDeleteExpected: false,
		},
		"DeleteValidationSuccess": {
			entryID:           1,
			checkID:           1,
			errGetExpected:    false,
			errDeleteExpected: false,
			deleteValidation: func(id uint16) error {
				return nil
			},
		},
		"DeleteValidationFailed": {
			entryID:           1,
			checkID:           1,
			errGetExpected:    true,
			errDeleteExpected: true,
			deleteValidation: func(id uint16) error {
				return fmt.Errorf("dummy error")
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries:      initEntries,
				DeleteValidation: tc.deleteValidation,
			})
			err := d.Delete(tc.entryID)
			if tc.errDeleteExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			e, err := d.Get(tc.checkID)
			if tc.errGetExpected {
				assert.NoError(t, err)
				if e != nil {

				} else {
					t.Errorf("TestGet: unexpected result, got nil, want: %v\n", e)
				}

			} else {
				assert.Error(t, err)
				if e != nil {
					t.Errorf("TestGet: unexpected result, got %v, want: nil\n", e)
				}
			}
		})
	}
}

func TestCount(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(1), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(4095), map[string]string{"a": "b"}),
	}

	cases := map[string]struct {
		initEntries     Entries[uint16]
		expectedEntries int
	}{
		"Count": {
			initEntries: initEntries,
			expectedEntries: 3,
		},
		"CountEmpty": {
			initEntries: Entries[uint16]{},
			expectedEntries: 0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries: tc.initEntries,
			})
			
			if d.Count() != tc.expectedEntries {
				t.Errorf("TestCount: unexpected result, got %d, want: %d\n", d.Count(), tc.expectedEntries)
			}
		})
	}
}

func TestIterate(t *testing.T) {
	initEntries := Entries[uint16]{
		NewEntry(uint16(0), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(1), map[string]string{"a": "b", "x": "y"}),
		NewEntry(uint16(4095), map[string]string{"a": "b"}),
	}

	cases := map[string]struct {
		initEntries     Entries[uint16]
		keys []uint16
	}{
		"Iterate": {
			initEntries: initEntries,
			keys: []uint16{0, 1, 4095},
		},
		"IterateEmpty": {
			initEntries: Entries[uint16]{},
			keys: []uint16{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := NewDB(&DBConfig[uint16]{
				InitEntries: tc.initEntries,
			})
			
			i :=  d.Iterate()
			if diff := cmp.Diff(tc.keys, i.keys); diff != "" {
				t.Errorf("TestIterate: -want, +got:\n%s", diff)
			}
		})
	}
}