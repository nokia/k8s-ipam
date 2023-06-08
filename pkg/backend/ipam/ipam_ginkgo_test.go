package ipam

import (
	"context"
	"encoding/json"

	resourcev1alpha1 "github.com/nokia/k8s-ipam/apis/resource/common/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/iputil"
	"github.com/nokia/k8s-ipam/pkg/meta"
	"github.com/nokia/k8s-ipam/pkg/utils/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("IPAM Testing", func() {
	var (
		ni = ipamv1alpha1.BuildNetworkInstance(
			metav1.ObjectMeta{
				Name:      "a",
				Namespace: "dummy",
			},
			ipamv1alpha1.NetworkInstanceSpec{},
			ipamv1alpha1.NetworkInstanceStatus{},
		)
		niBytes []byte
		be      backend.Backend
	)

	Context("When initing the ipam", func() {
		It("Should result in a usable ipam backend index", func() {
			By("calling New() constructor for an ipam backend")
			var err error
			// create new index
			be, err = New(nil)
			Ω(err).Should(Succeed(), "Failed to create ipam backend")

			Ω(be).ShouldNot(BeNil(), "initializing ipam failed")

			// create a new NI in the ipam
			niBytes, err = json.Marshal(ni)
			Ω(err).Should(Succeed(), "Failed to marshal network instance")
			err = be.CreateIndex(context.Background(), niBytes)
			Ω(err).Should(Succeed(), "Failed to create ipam index")
		})
	})
	Context("After adding the supernet", func() {
		It("should contain a single rib entry", func() {
			var err error
			Ω(be).ShouldNot(BeNil(), "initializing ipam failed")

			// test prefix
			prefix := ipamv1alpha1.Prefix{
				Prefix: "10.0.0.0/8",
			}
			pi, err := iputil.New(prefix.Prefix)
			Ω(err).Should(Succeed())
			// build claim for network instance prefix
			req := ipamv1alpha1.BuildIPClaim(
				metav1.ObjectMeta{
					Name:      ipamv1alpha1.GetNameFromPrefix(ni.Name, prefix.Prefix, ipamv1alpha1.NetworkInstancePrefixAggregate),
					Namespace: ni.Namespace,
					Labels: map[string]string{
						resourcev1alpha1.NephioOwnerGvkKey: meta.GVKToString(ipamv1alpha1.NetworkInstanceGroupVersionKind),
					},
				},
				ipamv1alpha1.IPClaimSpec{
					Kind:            ipamv1alpha1.PrefixKindAggregate,
					NetworkInstance: corev1.ObjectReference{Name: ni.Name, Namespace: ni.Namespace},
					Prefix:          pointer.String(prefix.Prefix),
					PrefixLength:    util.PointerUint8(pi.GetPrefixLength().Int()),
					CreatePrefix:    pointer.Bool(true),
					ClaimLabels: resourcev1alpha1.ClaimLabels{
						UserDefinedLabels: prefix.UserDefinedLabels,
					},
				},
				ipamv1alpha1.IPClaimStatus{},
			)
			req.AddOwnerLabelsToCR()
			Ω(req).ShouldNot(BeNil())
			b, err := json.Marshal(req)
			Ω(err).Should(Succeed(), "Failed to marshal claim req")
			rsp, err := be.Claim(context.Background(), b)
			Ω(err).Should(Succeed())
			resp := ipamv1alpha1.IPClaim{}
			err = json.Unmarshal(rsp, &resp)
			Ω(err).Should(Succeed(), "Failed to unmarshal claim resp")

			checkClaimResp(*req, resp, prefix.Prefix, "")

			// check rib entries
			Expect(be.List(context.Background(), niBytes)).To(ContainElements(ContainSubstring(prefix.Prefix)))
			Expect(be.List(context.Background(), niBytes)).To(HaveLen(1))
		})
	})
	Context("After adding the supernet, Add a network prefix", func() {
		It("should contain a multiple rib entry", func() {
			var err error
			Ω(be).ShouldNot(BeNil(), "initializing ipam failed")

			// test prefix
			prefix := ipamv1alpha1.Prefix{
				Prefix: "10.0.0.1/24",
				UserDefinedLabels: resourcev1alpha1.UserDefinedLabels{
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			}
			pi, err := iputil.New(prefix.Prefix)
			Ω(err).Should(Succeed())
			// // build claim for network kind prefix
			req := ipamv1alpha1.BuildIPClaim(
				metav1.ObjectMeta{
					Name:      "net-prefix-1",
					Namespace: ni.Namespace,
					Labels: map[string]string{
						resourcev1alpha1.NephioOwnerGvkKey: meta.GVKToString(ipamv1alpha1.IPPrefixGroupVersionKind),
					},
				},
				ipamv1alpha1.IPClaimSpec{
					Kind:            ipamv1alpha1.PrefixKindNetwork,
					NetworkInstance: corev1.ObjectReference{Name: ni.Name, Namespace: ni.Namespace},
					Prefix:          pointer.String(prefix.Prefix),
					PrefixLength:    util.PointerUint8(pi.GetPrefixLength().Int()),
					CreatePrefix:    pointer.Bool(true),
					ClaimLabels: resourcev1alpha1.ClaimLabels{
						UserDefinedLabels: prefix.UserDefinedLabels,
					},
				},
				ipamv1alpha1.IPClaimStatus{},
			)
			req.AddOwnerLabelsToCR()
			Ω(req).ShouldNot(BeNil())
			b, err := json.Marshal(req)
			Ω(err).Should(Succeed(), "Failed to marshal claim req")
			rsp, err := be.Claim(context.Background(), b)
			Ω(err).Should(Succeed())
			resp := ipamv1alpha1.IPClaim{}
			err = json.Unmarshal(rsp, &resp)
			Ω(err).Should(Succeed(), "Failed to unmarshal claim resp")

			checkClaimResp(*req, resp, prefix.Prefix, "")

			// check rib entries
			Expect(be.List(context.Background(), niBytes)).To(ContainElements(ContainSubstring(pi.GetFirstIPPrefix().String())))
			Expect(be.List(context.Background(), niBytes)).To(ContainElements(ContainSubstring(pi.GetFirstIPAddress().String())))
			Expect(be.List(context.Background(), niBytes)).To(ContainElements(ContainSubstring(pi.GetLastIPAddress().String())))
			Expect(be.List(context.Background(), niBytes)).To(ContainElements(ContainSubstring(pi.GetIPAddress().String())))
			Expect(be.List(context.Background(), niBytes)).To(HaveLen(5))
		})
	})
	Context("After adding the supernet/network prefix add a claim", func() {
		It("should contain a multiple rib entry", func() {
			var err error
			// test selector
			selector := &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"nephio.org/site": "edge1",
				},
			}

			// // build claim for network kind prefix
			req := ipamv1alpha1.BuildIPClaim(
				metav1.ObjectMeta{
					Name:      "claim-1",
					Namespace: ni.Namespace,
				},
				ipamv1alpha1.IPClaimSpec{
					Kind:            ipamv1alpha1.PrefixKindNetwork,
					NetworkInstance: corev1.ObjectReference{Name: ni.Name, Namespace: ni.Namespace},
					ClaimLabels: resourcev1alpha1.ClaimLabels{
						Selector: selector,
					},
				},
				ipamv1alpha1.IPClaimStatus{},
			)
			req.AddOwnerLabelsToCR()
			Ω(req).ShouldNot(BeNil())
			b, err := json.Marshal(req)
			Ω(err).Should(Succeed(), "Failed to marshal claim req")
			rsp, err := be.Claim(context.Background(), b)
			Ω(err).Should(Succeed())
			resp := ipamv1alpha1.IPClaim{}
			err = json.Unmarshal(rsp, &resp)
			Ω(err).Should(Succeed(), "Failed to unmarshal claim resp")

			checkClaimResp(*req, resp, "10.0.0.0", "10.0.0.1")

			// check rib entries
			Expect(be.List(context.Background(), niBytes)).To(HaveLen(6))
		})
	})
})

func checkClaimResp(req ipamv1alpha1.IPClaim, resp ipamv1alpha1.IPClaim, prefix, gateway string) {
	if req.Spec.Prefix != nil {
		Expect(resp.Status.Prefix).To(BeEquivalentTo(req.Spec.Prefix))
	} else {
		if req.Spec.Kind == ipamv1alpha1.PrefixKindNetwork && gateway != "" {
			Expect(*resp.Status.Gateway).To(BeIdenticalTo(gateway))
		}
	}
}
