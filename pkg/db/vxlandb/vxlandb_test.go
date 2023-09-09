package vxlandb

import (
	"fmt"
	"testing"

	"github.com/nokia/k8s-ipam/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cases := map[string]struct {
		id          uint32
		expectedErr bool
	}{
		"New": {
			id:          1111,
			expectedErr: false,
		},
		"AboveMax": {
			id:          1000000000,
			expectedErr: true,
		},
		"Max": {
			id:          65536,
			expectedErr: false,
		},
		"Max+1": {
			id:          65537,
			expectedErr: true,
		},
		"OK": {
			id:          10000,
			expectedErr: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := New(&Config[uint32]{Offset: 100, MaxEntryID: 65536})
			err := d.Set(db.NewEntry(tc.id, nil))
			if !tc.expectedErr {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			err = d.Delete(tc.id)
			if !tc.expectedErr {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestFree(t *testing.T) {
	cases := map[string]struct {
	}{
		"Free": {},
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			d := New(&Config[uint32]{Offset: 100, MaxEntryID: 65536})
			var e db.Entry[uint32]
			var err error
			e, err = d.FindFree()
			if err != nil {
				assert.Error(t, err)
			}
			fmt.Println(e)
			if err := d.Set(e); err != nil {
				assert.NoError(t, err)
			}
			e, err = d.FindFree()
			if err != nil {
				assert.NoError(t, err)
			}
			fmt.Println(e)
			err = d.Delete(e.ID())
			if err != nil {
				assert.NoError(t, err)
			}
		})
	}
}
