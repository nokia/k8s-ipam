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

package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	//_ "github.com/nokia/k8s-ipam/controllers/ipamspecializer"
	"github.com/henderiw-nephio/network-node-operator/pkg/node"
	"github.com/henderiw-nephio/network-node-operator/pkg/node/srlinux"
	_ "github.com/nokia/k8s-ipam/controllers/ipclaim"
	_ "github.com/nokia/k8s-ipam/controllers/ipnetworkinstance"
	_ "github.com/nokia/k8s-ipam/controllers/ipprefix"
	_ "github.com/nokia/k8s-ipam/controllers/node"
	_ "github.com/nokia/k8s-ipam/controllers/rawtopology"
	_ "github.com/nokia/k8s-ipam/controllers/vlanclaim"
	_ "github.com/nokia/k8s-ipam/controllers/vlanindex"

	//_ "github.com/nokia/k8s-ipam/controllers/vlanspecializer"
	_ "github.com/nokia/k8s-ipam/controllers/vlanvlan"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nephio-project/nephio-controller-poc/pkg/porch"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/resource/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/controllers/ctrlconfig"
	"github.com/nokia/k8s-ipam/internal/grpcserver"
	"github.com/nokia/k8s-ipam/internal/healthhandler"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/backend/ipam"
	"github.com/nokia/k8s-ipam/pkg/backend/vlan"
	"github.com/nokia/k8s-ipam/pkg/proxy/clientproxy"
	ipamcp "github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/ipam"
	vlancp "github.com/nokia/k8s-ipam/pkg/proxy/clientproxy/vlan"
	"github.com/nokia/k8s-ipam/pkg/proxy/serverproxy"
	//+kubebuilder:scaffold:imports
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "cannot initializer schema")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                     scheme,
		MetricsBindAddress:         metricsAddr,
		Port:                       9443,
		HealthProbeBindAddress:     probeAddr,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "3118b7ab.nephio.org",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("setup controller")
	ctx := ctrl.SetupSignalHandler()

	porchClient, err := porch.CreateClient()
	if err != nil {
		setupLog.Error(err, "unable to create porch client")
		os.Exit(1)
	}

	ctrlCfg := &ctrlconfig.ControllerConfig{
		Address: os.Getenv("RESOURCE_BACKEND"),
		IpamClientProxy: ipamcp.New(ctx, clientproxy.Config{
			Address: os.Getenv("RESOURCE_BACKEND"),
		}),
		VlanClientProxy: vlancp.New(ctx, clientproxy.Config{
			Address: os.Getenv("RESOURCE_BACKEND"),
		}),
		Noderegistry: registerSupportedNodeProviders(),
		PorchClient:  porchClient,
		Poll:         5 * time.Second,
		Copts: controller.Options{
			MaxConcurrentReconciles: 1,
		},
	}

	gevents := map[schema.GroupVersionKind]chan event.GenericEvent{}
	for name, reconciler := range controllers.Reconcilers {
		setupLog.Info("reconciler", "name", name, "enabled", IsReconcilerEnabled(name))
		if IsReconcilerEnabled(name) {
			e, err := reconciler.Setup(ctx, mgr, ctrlCfg)
			if err != nil {
				setupLog.Error(err, "cannot add controllers to manager")
				os.Exit(1)
			}
			for gvk, ech := range e {
				gevents[gvk] = ech
			}
		}
	}
	ctrlCfg.IpamClientProxy.AddEventChs(gevents)

	ipambe, err := ipam.New(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "cannot instantiate ipam backend")
		os.Exit(1)
	}
	vlanbe, err := vlan.New(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "cannot instantiate vlan backend")
		os.Exit(1)
	}

	ipamServerProxy := serverproxy.New(&serverproxy.Config{
		Backends: map[schema.GroupVersion]backend.Backend{
			ipamv1alpha1.GroupVersion: ipambe,
			vlanv1alpha1.GroupVersion: vlanbe,
		},
	})
	wh := healthhandler.New()

	s := grpcserver.New(grpcserver.Config{
		Address:  ":" + strconv.Itoa(9999),
		Insecure: true,
	},
		grpcserver.WithCreateIndexHandler(ipamServerProxy.CreateIndex),
		grpcserver.WithDeleteIndexHandler(ipamServerProxy.DeleteIndex),
		grpcserver.WithGetClaimHandler(ipamServerProxy.GetClaim),
		grpcserver.WithClaimHandler(ipamServerProxy.Claim),
		grpcserver.WithDeleteClaimHandler(ipamServerProxy.DeleteClaim),
		grpcserver.WithWatchClaimHandler(ipamServerProxy.Watch),
		grpcserver.WithWatchHandler(wh.Watch),
		grpcserver.WithCheckHandler(wh.Check),
	)

	go func() {
		if err := s.Start(ctx); err != nil {
			setupLog.Error(err, "cannot start grpcserver")
			os.Exit(1)
		}
	}()

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func IsReconcilerEnabled(reconcilerName string) bool {
	if _, found := os.LookupEnv(fmt.Sprintf("ENABLE_%s", strings.ToUpper(reconcilerName))); found {
		return true
	}
	return false
}

func registerSupportedNodeProviders() node.NodeRegistry {
	nodeRegistry := node.NewNodeRegistry()
	srlinux.Register(nodeRegistry)

	return nodeRegistry
}
