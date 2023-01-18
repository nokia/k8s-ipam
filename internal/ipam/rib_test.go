package ipam

import (
	"testing"
)

func TestRibPass(t *testing.T) {
	myNetworInstance := "myni"
	// create new rib
	r := newIpamRib()
	// create new networkinstance
	r.create(myNetworInstance)
	// check ni is uninitilaized after creation
	b := r.isInitialized(myNetworInstance)
	if b {
		t.Errorf("NetworkInstance %q appears initialized although not explicitly initilaized", myNetworInstance)
	}
	// initializing Network Instance
	err := r.setInitialized(myNetworInstance)
	if err != nil {
		t.Errorf("%v occured on initializing NetworkInstance %s", err, myNetworInstance)
	}
	// check initialized again should be true now
	b = r.isInitialized(myNetworInstance)
	if !b {
		t.Errorf("NetworkInstance %q appears initialized although not explicitly initilaized", myNetworInstance)
	}
}

func TestRibIsInitilaizedNonExisting(t *testing.T) {
	myNetworInstance := "non-existing-NI"
	// create new rib
	r := newIpamRib()

	// check ni isInitialized on non existing NetworkInstance
	b := r.isInitialized(myNetworInstance)
	if b {
		t.Errorf("NetworkInstance %q appears initialized although not explicitly initilaized", myNetworInstance)
	}
}

func TestRibSetInitilaizedNonExisting(t *testing.T) {
	myNetworInstance := "non-existing-NI"
	// create new rib
	r := newIpamRib()

	// initializing Network Instance
	err := r.setInitialized(myNetworInstance)
	if err == nil {
		t.Errorf("setInitialized on non-existing NetworkInstance should raise an error")
	}
}

func TestGetRibPass(t *testing.T) {
	myNetworInstance := "my-NI"

	// create new rib
	r := newIpamRib()
	// create new networkinstance
	r.create(myNetworInstance)

	// initializing Network Instance
	err := r.setInitialized(myNetworInstance)
	if err != nil {
		t.Errorf("%v occured on initializing NetworkInstance %s", err, myNetworInstance)
	}
	// retrieve the rib
	rib, err := r.getRIB(myNetworInstance, true)
	if err != nil {
		t.Errorf("calling getRIB for %q failed with %v", myNetworInstance, err)
	}
	if rib == nil {
		t.Errorf("got a nil rib")
	}
}

func TestGetRibFail(t *testing.T) {
	myNetworInstance := "non-existing-NI"

	// create new rib
	r := newIpamRib()

	// retrieve the rib
	_, err := r.getRIB(myNetworInstance, false)
	if err == nil {
		t.Errorf("calling getRIB for %q failed with %v", myNetworInstance, err)
	}

	// retrieve the rib
	_, err = r.getRIB(myNetworInstance, true)
	if err == nil {
		t.Errorf("calling getRIB for %q failed with %v", myNetworInstance, err)
	}
}
