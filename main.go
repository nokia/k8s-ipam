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
	"os"
	"strconv"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	porchv1alpha1 "github.com/GoogleContainerTools/kpt/porch/api/porch/v1alpha1"
	"github.com/nephio-project/nephio-controller-poc/pkg/porch"
	ipamv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/ipam/v1alpha1"
	vlanv1alpha1 "github.com/nokia/k8s-ipam/apis/alloc/vlan/v1alpha1"
	"github.com/nokia/k8s-ipam/controllers"
	"github.com/nokia/k8s-ipam/internal/grpcserver"
	"github.com/nokia/k8s-ipam/internal/healthhandler"
	"github.com/nokia/k8s-ipam/internal/ipam"
	"github.com/nokia/k8s-ipam/internal/shared"
	"github.com/nokia/k8s-ipam/internal/vlanbackend"
	"github.com/nokia/k8s-ipam/pkg/backend"
	"github.com/nokia/k8s-ipam/pkg/proxy/serverproxy"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vlanv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ipamv1alpha1.AddToScheme(scheme))
	utilruntime.Must(porchv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

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

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "3118b7ab.nephio.org",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("setup controller")
	ctx := ctrl.SetupSignalHandler()

	/*
		reg, err := registrator.New(ctx, ctrl.GetConfigOrDie(), &registrator.Options{
			ServiceDiscovery:          discovery.ServiceDiscoveryTypeK8s,
			ServiceDiscoveryNamespace: os.Getenv("POD_NAMESPACE"),
		})
		if err != nil {
			setupLog.Error(err, "Cannot create registrator")
			os.Exit(1)
		}
	*/

	//podName := os.Getenv("POD_NAME")
	//if podName == "" {
	//podName := "local-resource-backend"
	//}
	//address := os.Getenv("POD_IP")
	//if address == "" {
	//address := "127.0.0.1"
	//}
	//namespace := os.Getenv("POD_NAMESPACE")
	//if namespace == "" {
	//	namespace = "resource-backend"
	//}

	// register the service
	/*
		go func() {
			reg.Register(ctx, &registrator.Service{
				Name:         "resource-backend",
				ID:           podName,
				Port:         9999,
				Address:      address,
				Tags:         []string{discovery.GetPodServiceTag(namespace, podName)},
				HealthChecks: []registrator.HealthKind{registrator.HealthKindGRPC, registrator.HealthKindTTL},
			})
		}()
	*/

	porchClient, err := porch.CreateClient()
	if err != nil {
		setupLog.Error(err, "unable to create porch client")
		os.Exit(1)
	}

	// initialize controllers
	if err := controllers.Setup(ctx, mgr, &shared.Options{
		Address: "127.0.0.1:9999",
		//Registrator: reg,
		PorchClient: porchClient,
		Poll:        5 * time.Second,
		Copts: controller.Options{
			MaxConcurrentReconciles: 1,
		},
	}); err != nil {
		setupLog.Error(err, "Cannot add controllers to manager")
		os.Exit(1)
	}

	ipambe, err := ipam.New(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "Cannot add controllers to manager")
		os.Exit(1)
	}
	vlanbe, err := vlanbackend.New(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "Cannot add controllers to manager")
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
		grpcserver.WithGetAllocHandler(ipamServerProxy.GetAllocation),
		grpcserver.WithAllocHandler(ipamServerProxy.Allocate),
		grpcserver.WithDeAllocHandler(ipamServerProxy.DeAllocate),
		grpcserver.WithWatchAllocHandler(ipamServerProxy.Watch),
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
