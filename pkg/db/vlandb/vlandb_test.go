package vlandb

import (
	"testing"

	"github.com/nokia/k8s-ipam/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	cases := map[string]struct {
		id          uint16
		expectedErr bool
	}{
		"New": {
			id:          1111,
			expectedErr: false,
		},
		"NewReserved1": {
			id:          1,
			expectedErr: true,
		},
		"NewReserved4095": {
			id:          4095,
			expectedErr: true,
		},
		"NewReserved40": {
			id:          0,
			expectedErr: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := New()
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
