package ipam

import (
	"context"

	"github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/ipam/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("IPAM Testing", func() {
	var (
		niRef = &ipamv1alpha1.NetworkInstanceReference{
			Namespace: "dummy",
			Name:      "niName",
		}
		allocNamespace = "test"
		niCreate       = &allocation{spec: ipamv1alpha1.IPAllocationSpec{NetworkInstanceRef: niRef}}
		ipam           Ipam
		netInstance    *v1alpha1.NetworkInstance
	)

	Context("When initing the ipam", func() {
		It("Should result in a usable ipam instance", func() {
			By("calling New() constructor for an Ipam")
			// create new rib
			ipam = New(nil)

			Ω(ipam).ShouldNot(BeNil(), "initializing ipam failed")

			// create new networkinstance
			netInstance = buildNetworkInstance(niCreate)

			// create a new NI in the ipam
			err := ipam.Create(context.Background(), netInstance)
			Ω(err).Should(Succeed(), "Failed to create ipam network instance")
		})
	})
	Context("After adding the supernet", func() {
		It("should contain a single rib entry", func() {
			a1 := &allocation{
				kind:      ipamv1alpha1.NetworkInstanceKind,
				namespace: niRef.Namespace,
				name:      niRef.Name,
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					Prefix:             "10.0.0.0/8",
				},
			}

			allocReq := buildIPAllocation(a1)
			Ω(allocReq).ShouldNot(BeNil())

			allocResp, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
			Ω(err).Should(Succeed())
			checkAllocResp(a1, *allocResp, "")

			// check rib entries
			Expect(ipam.GetPrefixes(netInstance)).To(ContainElements(r0))
			Expect(ipam.GetPrefixes(netInstance)).To(HaveLen(1))
		})
	})
	Context("After adding a subnet without corresponding supernet", func() {
		It("should error with errValidateNetworkPrefixWoNetworkParent", func() {
			a2 := &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.2/24",
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			}
			allocReq := buildIPAllocation(a2)
			Expect(allocReq).ShouldNot(BeNil())

			_, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
			Expect(err).Error()
			Expect(err.Error()).Should(ContainSubstring(errValidateNetworkPrefixWoNetworkParent))

			// check rib entries
			Expect(ipam.GetPrefixes(netInstance)).To(ContainElements(r0))
			Expect(ipam.GetPrefixes(netInstance)).To(HaveLen(1))
		})
	})
	Context("After creating a valid subnet", func() {
		It("should pass", func() {
			a3 := &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.1/24",
					CreatePrefix:       true,
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			}

			allocReq := buildIPAllocation(a3)
			Expect(allocReq).ShouldNot(BeNil())

			allocResp, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
			Expect(err).Should(Succeed())

			checkAllocResp(a3, *allocResp, "")

			// check rib entries
			Expect(ipam.GetPrefixes(netInstance)).To(ContainElements(r0, r1, r2, r3, r6))
			Expect(ipam.GetPrefixes(netInstance)).To(HaveLen(5))
		})
	})
	Context("After creating a duplicate", func() {
		It("should not be in the rib and an error should be raised", func() {
			a4 := &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-prefix2",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Prefix:             "10.0.0.10/24",
					CreatePrefix:       true,
					Labels: map[string]string{
						"nephio.org/gateway":      "true",
						"nephio.org/region":       "us-central1",
						"nephio.org/site":         "edge1",
						"nephio.org/network-name": "net1",
					},
				},
			}

			allocReq := buildIPAllocation(a4)
			Expect(allocReq).ShouldNot(BeNil())

			_, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
			Expect(err).ShouldNot(Succeed())

			// check rib entries
			Expect(ipam.GetPrefixes(netInstance)).To(ContainElements(r0, r1, r2, r3, r6))
			Expect(ipam.GetPrefixes(netInstance)).To(HaveLen(5))
		})
	})
	Context("After creating a valid allocation", func() {
		It("should be present in the rib and have a gateway label set", func() {
			a5 := &allocation{
				kind:      ipamv1alpha1.IPAllocationKind,
				namespace: allocNamespace,
				name:      "alloc-net1-alloc1",
				spec: ipamv1alpha1.IPAllocationSpec{
					NetworkInstanceRef: niRef,
					PrefixKind:         ipamv1alpha1.PrefixKindNetwork,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"nephio.org/region":       "us-central1",
							"nephio.org/site":         "edge1",
							"nephio.org/network-name": "net1",
						},
					},
				},
			}

			allocReq := buildIPAllocation(a5)
			Expect(allocReq).ShouldNot(BeNil())

			allocResp, err := ipam.AllocateIPPrefix(context.Background(), allocReq)
			Expect(err).Should(Succeed())

			checkAllocResp(a5, *allocResp, "10.0.0.1")

			foo := ipam.GetPrefixes(netInstance)
			_ = foo

			// check rib entries
			Expect(ipam.GetPrefixes(netInstance)).To(HaveLen(6))
			Expect(ipam.GetPrefixes(netInstance)).To(ContainElements(r0, r1, r2, r3, r5, r6))
		})
	})
})

func checkAllocResp(req *allocation, resp ipamv1alpha1.IPAllocation, gateway string) {
	if req.spec.Prefix != "" {
		Expect(resp.Status.AllocatedPrefix).To(BeEquivalentTo(req.spec.Prefix))
	} else {
		Expect(resp.Status.Gateway).To(BeIdenticalTo(gateway))
	}
}
