package vlan

import (
	"context"
	"encoding/json"

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("VLAN Backend Testing", func() {
	var (
		db = vlanv1alpha1.BuildVLANIndex(
			metav1.ObjectMeta{
				Name:      "a",
				Namespace: "dummy",
			},
			vlanv1alpha1.VLANIndexSpec{},
			vlanv1alpha1.VLANIndexStatus{},
		)
		dbBytes []byte
		be      backend.Backend
	)

	Context("When initing the vlan backend", func() {
		It("Should result in a usable vlan backend index", func() {
			By("calling New() constructor for an ipam backend")
			var err error
			// create new backend
			be, err = New(nil)
			Ω(err).Should(Succeed(), "Failed to create backend")
			Ω(be).ShouldNot(BeNil(), "initializing backend failed")

			// create a new backend index
			dbBytes, err = json.Marshal(db)
			Ω(err).Should(Succeed(), "Failed to marshal backend index")
			err = be.CreateIndex(context.Background(), dbBytes)
			Ω(err).Should(Succeed(), "Failed to create backend index")
		})
	})
	Context("After adding a static VLAN", func() {
		It("should contain a single entry", func() {
			var err error

			// test element
			vlanID := 100
			// build claim for network instance prefix
			req := vlanv1alpha1.BuildVLANClaim(
				metav1.ObjectMeta{
					Name:      "static-vlan1",
					Namespace: db.Namespace,
					Labels: map[string]string{
						resourcev1alpha1.NephioOwnerGvkKey: meta.GVKToString(vlanv1alpha1.VLANGroupVersionKind),
					},
				},
				vlanv1alpha1.VLANClaimSpec{
					VLANIndex: corev1.ObjectReference{Name: db.Name, Namespace: db.Namespace},
					VLANID:    util.PointerUint16(uint16(vlanID)),
				},
				vlanv1alpha1.VLANClaimStatus{},
			)
			req.AddOwnerLabelsToCR()
			Ω(req).ShouldNot(BeNil())
			b, err := json.Marshal(req)
			Ω(err).Should(Succeed(), "Failed to marshal claim req")
			rsp, err := be.Claim(context.Background(), b)
			Ω(err).Should(Succeed())
			resp := vlanv1alpha1.VLANClaim{}
			err = json.Unmarshal(rsp, &resp)
			Ω(err).Should(Succeed(), "Failed to unmarshal claim resp")

			checkClaimResp(*req, resp)

			// check rib entries
			Expect(be.List(context.Background(), dbBytes)).To(HaveLen(4))
		})
	})
	Context("After adding the static vlan, Add a dynamic vlan", func() {
		It("should contain a multiple rib entry", func() {
			var err error

			// // build claim for network kind prefix
			req := vlanv1alpha1.BuildVLANClaim(
				metav1.ObjectMeta{
					Name:      "dynamic-vlan1",
					Namespace: db.Namespace,
				},
				vlanv1alpha1.VLANClaimSpec{
					VLANIndex: corev1.ObjectReference{Name: db.Name, Namespace: db.Namespace},
				},
				vlanv1alpha1.VLANClaimStatus{},
			)
			req.AddOwnerLabelsToCR()
			Ω(req).ShouldNot(BeNil())
			b, err := json.Marshal(req)
			Ω(err).Should(Succeed(), "Failed to marshal claim req")
			rsp, err := be.Claim(context.Background(), b)
			Ω(err).Should(Succeed())
			resp := vlanv1alpha1.VLANClaim{}
			err = json.Unmarshal(rsp, &resp)
			Ω(err).Should(Succeed(), "Failed to unmarshal claim resp")

			checkClaimResp(*req, resp)

			// check rib entries
			Expect(be.List(context.Background(), dbBytes)).To(HaveLen(5))
		})
	})
})

func checkClaimResp(req vlanv1alpha1.VLANClaim, resp vlanv1alpha1.VLANClaim) {
	if req.Spec.VLANID != nil {
		Expect(*resp.Status.VLANID).To(BeIdenticalTo(*req.Spec.VLANID))
	}
}
