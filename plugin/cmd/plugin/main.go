package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/dspinhirne/netaddr-go"
	"github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	"github.com/nokia/k8s-ipam/plugin/build"
	"github.com/nokia/k8s-ipam/plugin/pkg/config"
	"github.com/nokia/k8s-ipam/plugin/pkg/kubernetes"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ipamOwner        = "owner.ipam.nephio.org"
	ipamPlugin       = "k8s-ipam"
	IPPrefixLabelKey = "ipPrefix"
)

func main() {
	skel.PluginMain(func(args *skel.CmdArgs) error {
		ipamConf, err := config.LoadIPAMConfig(args.StdinData, args.Args)
		if err != nil {
			return err
		}

		k8sClient, err := kubernetes.NewKubernetesIPAM(ipamConf.Kubeconfig)
		if err != nil {
			return logging.Errorf("failed to create Kubernetes IPAM client: %v", err)
		}

		return cmdAdd(args, k8sClient, ipamConf)
	},
		cmdCheck,
		func(args *skel.CmdArgs) error {
			ipamConf, err := config.LoadIPAMConfig(args.StdinData, args.Args)
			if err != nil {
				return err
			}

			ipam, err := kubernetes.NewKubernetesIPAM(ipamConf.Kubeconfig)
			if err != nil {
				return logging.Errorf("failed to create kuberntes IPAM client: %v", err)
			}

			return cmdDel(args, ipam, ipamConf)
		},
		version.All,
		fmt.Sprintf("k8s-ipam %s", build.Version),
	)
}

func cmdAdd(args *skel.CmdArgs, k8sClient client.Client, ipamConf *config.IPAMConfig) error {
	k8sArgs := &kubernetes.K8sArgs{}

	err := cnitypes.LoadArgs(args.Args, k8sArgs)
	if err != nil {
		return fmt.Errorf("failed to parse CNI args: %w", err)
	}

	podName := string(k8sArgs.K8S_POD_NAME)
	podNamespace := string(k8sArgs.K8S_POD_NAMESPACE)
	if podName == "" || podNamespace == "" {
		return fmt.Errorf("podName and Namespace must be non nil. %v. %v", podName, podNamespace)
	}

	claimName := podName
	if ipamConf.IPPrefix != "" {
		claimName += "-" + ipamConf.IPPrefix
	}

	// Look for an IP allocation for pod
	claim := &v1alpha1.IPClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: podNamespace,
		},
		Spec: v1alpha1.IPClaimSpec{
			Kind: v1alpha1.PrefixKindNetwork,
			NetworkInstance: corev1.ObjectReference{
				Name:      ipamConf.NetInstance,
				Namespace: ipamConf.NetNamespace,
			},
		},
	}

	// If a network instance having multiple prefixes, net-attach-def may suggest the prefix to use
	if ipamConf.IPPrefix != "" {
		claim.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				IPPrefixLabelKey: ipamConf.IPPrefix,
			},
		}
	}

	err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: podNamespace, Name: claimName}, claim)
	if err != nil && errors.IsNotFound(err) {
		// Create a new IP claim
		claim.Annotations = map[string]string{
			ipamOwner: ipamPlugin,
		}

		cerr := k8sClient.Create(context.Background(), claim)
		if cerr != nil && !errors.IsAlreadyExists(cerr) {
			return cerr
		}
	}

	for i := 0; i < 10; i++ {
		err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: podNamespace, Name: claimName}, claim)
		if err == nil {
			// If not provisioned yet wait until k8s-ipam provisoined it.
			if claim.Status.Prefix == nil {
				time.Sleep(time.Second)

				continue
			}

			result := &current.Result{CNIVersion: ipamConf.CNIVersion}
			if claim.Status.Prefix != nil {
				prefixIP, prefix, err := net.ParseCIDR(*claim.Status.Prefix)
				if err != nil {
					return err
				}

				ip := net.IPNet{
					IP:   prefixIP,
					Mask: prefix.Mask,
				}

				netAddr, err := netaddr.ParseIPv4Net(prefix.String())
				if err != nil {
					return err
				}

				gw := net.ParseIP(netAddr.Nth(1).String())

				result.IPs = append(result.IPs, &current.IPConfig{
					Address: ip,
					Gateway: gw,
				})

				return cnitypes.PrintResult(result, current.ImplementedSpecVersion)
			}
		}

		return err
	}

	return fmt.Errorf("k8s-ipam failed to provision claim: %v", claim.Status.ConditionedStatus)
}

func cmdDel(args *skel.CmdArgs, k8sClient client.Client, ipamConf *config.IPAMConfig) error {
	k8sArgs := &kubernetes.K8sArgs{}

	err := cnitypes.LoadArgs(args.Args, k8sArgs)
	if err != nil {
		return fmt.Errorf("failed to parse CNI args: %w", err)
	}

	podName := string(k8sArgs.K8S_POD_NAME)
	podNamespace := string(k8sArgs.K8S_POD_NAMESPACE)
	if podName == "" || podNamespace == "" {
		return fmt.Errorf("podName and Namespace must be non nil. %v. %v", podName, podNamespace)
	}

	claimName := podName
	if ipamConf.IPPrefix != "" {
		claimName += "-" + ipamConf.IPPrefix
	}

	// Look for an IP allocation for pod
	claim := &v1alpha1.IPClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: podNamespace,
		},
	}
	err = k8sClient.Get(context.Background(), types.NamespacedName{Namespace: podNamespace, Name: claimName}, claim)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// If IPAM is the owner delete the claim
	if _, ok := claim.Annotations[ipamOwner]; ok {
		err = k8sClient.Delete(context.Background(), claim)
		if err != nil {
			return err
		}
	}

	return nil
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}
