/*
Copyright 2022 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vlan

/*
func buildVlanDatabase(name string) *vlanv1alpha1.VLANDatabase {
	return &vlanv1alpha1.VLANDatabase{
		TypeMeta: metav1.TypeMeta{
			APIVersion: vlanv1alpha1.GroupVersion.String(),
			Kind:       vlanv1alpha1.VLANDatabaseKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: vlanv1alpha1.VLANDatabaseSpec{
			Kind: "esg",
		},
	}
}

func buildDynamicVlanClaimation() *vlanv1alpha1.VLANClaimation {
	return vlanv1alpha1.BuildVLANClaimationFromVLANClaimation(
		&vlanv1alpha1.VLANClaimation{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vlanv1alpha1.GroupVersion.String(),
				Kind:       vlanv1alpha1.VLANDatabaseKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "dyn1",
			},
			Spec: vlanv1alpha1.VLANClaimationSpec{
				VLANDatabases: []*corev1.ObjectReference{
					{
						Kind:      "esg",
						Name:      "test",
						Namespace: "default",
					},
				},
				Labels: map[string]string{
					"a": "b",
				},
			},
		},
	)
}

func buildDynamicVlanClaimationSelector() *vlanv1alpha1.VLANClaimation {
	return vlanv1alpha1.BuildVLANClaimationFromVLANClaimation(
		&vlanv1alpha1.VLANClaimation{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vlanv1alpha1.GroupVersion.String(),
				Kind:       vlanv1alpha1.VLANDatabaseKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "dyn1",
			},
			Spec: vlanv1alpha1.VLANClaimationSpec{
				VLANDatabases: []*corev1.ObjectReference{
					{
						Kind:      "esg",
						Name:      "test",
						Namespace: "default",
					},
				},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"a": "b",
					},
				},
			},
		},
	)
}

func TestDynamicVlan(t *testing.T) {
	be, err := New(nil)
	if err != nil {
		t.Error("cannot initialize vlan backend")
	}
	ctx := context.Background()
	vlandbCr := buildVlanDatabase("test")
	if err := be.Create(ctx, vlandbCr); err != nil {
		t.Error("cannot create vlan db")
	}

	claim := buildDynamicVlanClaimation()
	claim, err = be.Claimate(ctx, claim)
	if err != nil {
		t.Error("cannot create claim")
	}
	fmt.Printf("claim: %v \n", claim.Status.ClaimatedVlanID)

	claim = buildDynamicVlanClaimationSelector()
	claim, err = be.Get(ctx, claim)
	if err != nil {
		t.Error("cannot get claim")
	}

	fmt.Printf("claim: %v \n", claim.Status)
}
*/
