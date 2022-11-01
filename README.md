# ipam

The IPAM is a kubernetes native IP address management, which supports:
- virtual networks to allow for overlapping IP(s)
- IP addresses, IP prefixes, IP pools and IP ranges within a virtual network
- A k8s api for configuring and allocating IP addresses within a virtual network
- A GRPC API for allocating and deallocating IP addresses/prefixes/pools
- labels as selectors for IP address allocation or to provide metadata to the object
- IPv6 and IPv4 in single stack or dual stack mode

-> add picture

## ipam logic and terminology

The IPAM has multiple network contexts (implemented as network-instances) that can have multiple prefixes that can be nested. The top prefix of a nested hierarchy is called an aggregated prefix. At the bottom layer we can have IP ranges or IP addresses that are allocated from within a prefix.

TODO add picture

Prefix - A subnet defined within an aggregate prefix. Prefixes extend the hierarchy by nesting within one another. (For example, 2000:1:1::/64 will appear within 2000:1::/48.) 

IP Range - An arbitrary range of individual IP addresses within a prefix, all sharing the same mask. (out of scope for now)

IP Address - An individual IP address along with its subnet mask, automatically arranged beneath its parent prefix.

The actual IPPrefix CRD does not distinguish between an address or a prefix, since an address is a special case of a prefix. An address has a /128 or /32 for ipv6, ipv4 resp.

## proxy

Besides the base IPAM block there is also a proxy functions which looks at IP Allocations within a GitRepo/package revision and allocates/deallocates IP(s) using a GRPC interface. This is a pluggable system which allows to interact with 3rd party IPAM systems.

## use cases

### Setup IPAM

To steup the IPAM, one needs to configure a virtual network, implemented through a network-instance

```
cat <<EOF | kubectl apply -f -
apiVersion: ipam.nephio.org/v1alpha1
kind: NetworkInstance
metadata:
  labels:
    app.kubernetes.io/created-by: ipam
  name: network-1
EOF
```

The next step is to create a prefix from which ip addresses can be allocated

```
cat <<EOF | kubectl apply -f -
apiVersion: ipam.nephio.org/v1alpha1
kind: IPPrefix
metadata:
  labels:
    app.kubernetes.io/name: ipprefix
    app.kubernetes.io/instance: network1-prefix1
    app.kubernetes.io/part-of: ipam
    app.kubernetes.io/created-by: ipam
    nephio.org/cluster: nephio-edge-01
  name: network1-prefix1
spec:
  prefix: 10.0.0.1/24
  networkInstance: network-1
EOF
```

```
cat <<EOF | kubectl apply -f -
apiVersion: ipam.nephio.org/v1alpha1
kind: IPPrefix
metadata:
  labels:
    app.kubernetes.io/name: ipprefix
    app.kubernetes.io/instance: network1-prefix2
    app.kubernetes.io/part-of: ipam
    app.kubernetes.io/created-by: ipam
    nephio.org/cluster: nephio-edge-01
  name: network1-prefix2
spec:
  prefix: 10.0.0.0/16
  networkInstance: network-1
EOF
```

To verify the status in the system we can use the following command

```
kubectl get ipam
```

The output will look like this

```
NAME                                        SYNC   STATUS   NETWORK     PREFIX        AGE
ipprefix.ipam.nephio.org/network1-prefix1   True   True     network-1   10.0.0.1/24   14s
ipprefix.ipam.nephio.org/network1-prefix2   True   True     network-1   10.0.0.0/16   3s

NAME                                        SYNC   STATUS   AGE
networkinstance.ipam.nephio.org/network-1   True   True     2m57s
```

To view the IPAM IP allocation we can look at the allocations under the network-instance

```
kubectl describe  networkinstances.ipam.nephio.org network-1
```

The output will look like this

```
k describe networkinstances.ipam.nephio.org network-1
Name:         network-1
Namespace:    default
API Version:  ipam.nephio.org/v1alpha1
Kind:         NetworkInstance
Status:
  Allocations:
    10.0.0.0/16:
      app.kubernetes.io/created-by:  ipam
      app.kubernetes.io/instance:    network1-prefix2
      app.kubernetes.io/name:        ipprefix
      app.kubernetes.io/part-of:     ipam
      nephio.org/address-family:     ipv4
      nephio.org/allocation-name:    network1-prefix2
      nephio.org/cluster:            nephio-edge-01
      nephio.org/network-instance:   network-1
      nephio.org/origin:             prefix
      nephio.org/prefix-length:      16
      nephio.org/prefix-name:        network1-prefix2
    10.0.0.0/24:
      app.kubernetes.io/created-by:  ipam
      app.kubernetes.io/instance:    network1-prefix1
      app.kubernetes.io/name:        ipprefix
      app.kubernetes.io/part-of:     ipam
      nephio.org/address-family:     ipv4
      nephio.org/allocation-name:    network1-prefix1
      nephio.org/cluster:            nephio-edge-01
      nephio.org/network-instance:   network-1
      nephio.org/origin:             prefix
      nephio.org/prefix-length:      24
      nephio.org/prefix-name:        network1-prefix1
    10.0.0.1/32:
      app.kubernetes.io/created-by:     ipam
      app.kubernetes.io/instance:       network1-prefix1
      app.kubernetes.io/name:           ipprefix
      app.kubernetes.io/part-of:        ipam
      nephio.org/address-family:        ipv4
      nephio.org/allocation-name:       network1-prefix1
      nephio.org/cluster:               nephio-edge-01
      nephio.org/gateway:               true
      nephio.org/network-instance:      network-1
      nephio.org/origin:                prefix
      nephio.org/parent-net:            10.0.0.0
      nephio.org/parent-prefix-length:  24
      nephio.org/prefix-length:         24
      nephio.org/prefix-name:           network1-prefix1
  Conditions:
    Kind:                  Synced
    Last Transition Time:  2022-11-01T07:37:51Z
    Reason:                ReconcileSuccess
    Status:                True
    Kind:                  Ready
    Last Transition Time:  2022-11-01T07:37:51Z
    Reason:                Ready
    Status:                True
Events:                    <none>
```

### IP address allocation 

To request an IP address from the IPAM system we either use the K8s or the GRPC API.
By providing a network-instance and prefix-name label-selector an IP address will be allocated
from an IPAM prefix that matches these labels.

```
cat <<EOF | kubectl apply -f -
apiVersion: ipam.nephio.org/v1alpha1
kind: IPAllocation
metadata:
 name: ipalloc
spec:
 selector:
   matchLabels:
     nephio.org/network-instance: network-1
     nephio.org/prefix-name: network1-prefix1
EOF
```

A prefix and parent prefix is allocated

```
NAME                                   SYNC   STATUS   PREFIX        PARENTPREFIX   AGE
ipallocation.ipam.nephio.org/ipalloc   True   True     10.0.0.0/32   10.0.0.0/24    6s
```

### static IP address allocation 

To support static or determinsitic IP allocation a predetermined IP is allocated using the IP Prefix API, that sets a specific label e.g. network1-prefix1-n3

```
cat <<EOF | kubectl apply -f -
apiVersion: ipam.nephio.org/v1alpha1
kind: IPPrefix
metadata:
  labels:
    nephio.org/interface: n3
  name: network1-prefix1-n3
spec:
  prefix: 10.0.0.2/32
  networkInstance: network-1
EOF
```

By referencing this label in the label selector we can allocate the IP that was statically allocated

```
cat <<EOF | kubectl apply -f -
apiVersion: ipam.nephio.org/v1alpha1
kind: IPAllocation
metadata:
  name: ipalloc-n3
spec:
  selector:
    matchLabels:
      nephio.org/network-instance: network-1
      nephio.org/prefix-name: network1-prefix1
      nephio.org/interface: n3
EOF
```

```
NAME                                           SYNC   STATUS   NETWORK     PREFIX        AGE
ipprefix.ipam.nephio.org/network1-prefix1      True   True     network-1   10.0.0.1/24   16m
ipprefix.ipam.nephio.org/network1-prefix1-n3   True   True     network-1   10.0.0.2/32   2m43s
ipprefix.ipam.nephio.org/network1-prefix2      True   True     network-1   10.0.0.0/16   16m

NAME                                        SYNC   STATUS   AGE
networkinstance.ipam.nephio.org/network-1   True   True     18m

NAME                                      SYNC   STATUS   PREFIX        PARENTPREFIX   AGE
ipallocation.ipam.nephio.org/ipalloc      True   True     10.0.0.0/32   10.0.0.0/24    5m27s
ipallocation.ipam.nephio.org/ipalloc-n3   True   True     10.0.0.2/32   10.0.0.0/24    13s
```

### GW IP address allocation

To request the GW IP of the prefix, we use the following mechanism.

First when creating the IP prefix we create it using the following notation 10.0.0.1/24. As such the .1 is allocated by the IPAM as a gateway IP automatically.

Allocate an IP with the ephio.org/gateway: "true" label in the label selector

```
apiVersion: ipam.nephio.org/v1alpha1
kind: IPAllocation
metadata:
  name: ipalloc-gw
spec:
  selector:
    matchLabels:
      nephio.org/network-instance: network-1
      nephio.org/prefix-name: network1-prefix1
      nephio.org/gateway: "true"
```

Now also the GW IP will be referenced

```
henderiw@henderiookpro16 samples % k get ipam
NAME                                           SYNC   STATUS   NETWORK     PREFIX        AGE
ipprefix.ipam.nephio.org/network1-prefix1      True   True     network-1   10.0.0.1/24   19m
ipprefix.ipam.nephio.org/network1-prefix1-n3   True   True     network-1   10.0.0.2/32   5m58s
ipprefix.ipam.nephio.org/network1-prefix2      True   True     network-1   10.0.0.0/16   19m

NAME                                        SYNC   STATUS   AGE
networkinstance.ipam.nephio.org/network-1   True   True     22m

NAME                                      SYNC   STATUS   PREFIX        PARENTPREFIX   AGE
ipallocation.ipam.nephio.org/ipalloc      True   True     10.0.0.0/32   10.0.0.0/24    8m42s
ipallocation.ipam.nephio.org/ipalloc-gw   True   True     10.0.0.1/32   10.0.0.0/24    2s
ipallocation.ipam.nephio.org/ipalloc-n3   True   True     10.0.0.2/32   10.0.0.0/24    3m28s
```

### pool allocation

To allocate a pool we speific the prefix length and the network-instance that should match

```
apiVersion: ipam.nephio.org/v1alpha1
kind: IPAllocation
metadata:
  name: pool
spec:
  prefixLength: 23
  selector:
    matchLabels:
      nephio.org/network-instance: network-1
```

```
NAME                                        SYNC   STATUS   AGE
networkinstance.ipam.nephio.org/network-1   True   True     25m

NAME                                           SYNC   STATUS   NETWORK     PREFIX        AGE
ipprefix.ipam.nephio.org/network1-prefix1      True   True     network-1   10.0.0.1/24   23m
ipprefix.ipam.nephio.org/network1-prefix1-n3   True   True     network-1   10.0.0.2/32   9m30s
ipprefix.ipam.nephio.org/network1-prefix2      True   True     network-1   10.0.0.0/16   22m

NAME                                      SYNC   STATUS   PREFIX        PARENTPREFIX   AGE
ipallocation.ipam.nephio.org/ipalloc      True   True     10.0.0.0/32   10.0.0.0/24    12m
ipallocation.ipam.nephio.org/ipalloc-gw   True   True     10.0.0.1/32   10.0.0.0/24    3m34s
ipallocation.ipam.nephio.org/ipalloc-n3   True   True     10.0.0.2/32   10.0.0.0/24    7m
ipallocation.ipam.nephio.org/pool         True   True     10.0.2.0/23                  58s
```
## License

Copyright 2022 nephio.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

