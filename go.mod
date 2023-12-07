module github.com/nokia/k8s-ipam

go 1.20

//replace k8s.io/api => k8s.io/api v0.26.1

//replace k8s.io/client-go => k8s.io/client-go v0.26.1

//replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.14.6

require (
	github.com/go-logr/logr v1.2.4
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.5.9
	github.com/hansthienpondt/nipam v0.0.5
	github.com/henderiw-nephio/network-node-operator v0.0.0-20230725011206-3110827f2100
	github.com/nephio-project/nephio-controller-poc v0.0.0-20221111013453-5a31b4722094
	github.com/nephio-project/nephio/controllers/pkg v0.0.0-20230629025102-1d5a6bc22053
	github.com/onsi/ginkgo/v2 v2.11.0
	github.com/onsi/gomega v1.27.10
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.6.1
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.24.0
	go4.org/netipx v0.0.0-20230303233057-f1b76eb4bb35
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29
	golang.org/x/sync v0.3.0
	google.golang.org/grpc v1.57.0
	k8s.io/api v0.27.4
	k8s.io/apimachinery v0.27.4
	k8s.io/client-go v0.27.4
	k8s.io/component-base v0.27.2
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	sigs.k8s.io/apiserver-runtime v1.1.1
	sigs.k8s.io/controller-runtime v0.15.0
	sigs.k8s.io/kustomize/kyaml v0.14.2
	sigs.k8s.io/yaml v1.3.0
)

require (
	go.etcd.io/etcd/client/pkg/v3 v3.5.9 // indirect
	go.etcd.io/etcd/client/v3 v3.5.9 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230803162519-f966b187b2e5 // indirect
	k8s.io/apiserver v0.27.2 // indirect
)

require (
	github.com/GoogleContainerTools/kpt/porch/api v0.0.0-20230608012444-ee7c8cf378e9 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/creack/pty v1.1.18 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20230509042627-b1315fad0c5a // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.4.0 // indirect
	github.com/kentik/patricia v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/scrapli/scrapligo v1.1.10 // indirect
	github.com/sirikothe/gotextfsm v1.0.1-0.20200816110946-6aa2cfd355e4 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/crypto v0.12.0 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/oauth2 v0.11.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/term v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.9.3 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230807174057-1744710a1577 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.27.2 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230525220651-2546d827e515 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
