# configuring a raw topology

The resource backend offers a convenient way to provision a topology using the RawTopology Custom Resource Definition (CRD). This abstraction layer provides a simplified experience for consumers who prefer not to interact directly with individual low-level CRDs, such as node, link, and endpoint.

The RawTopology CRD is primarily designed to define topologies in a declarative manner, making it ideal for setting up labs or running systems. The output generated from the RawTopology CRD serves multiple purposes, including:
- Spinning up and interconnecting labs or virtual environments, streamlining the process of creating complex network setups.
- Assisting inventory systems, allowing for efficient tracking and management of various network components and configurations.
- Enhancing provisioning systems, providing the necessary inputs for automated deployment and configuration of networks and services.


## example

The `RawTopology` CRD is inspired by [containerlab](https://containerlab.dev) [Topology definition](https://containerlab.dev/manual/topo-def-file/), but are more tailored towards kubernetes and have a more extensive schema behind. 

```yaml
cat <<EOF | kubectl apply -f - 
apiVersion: topo.nephio.org/v1alpha1
kind: RawTopology
metadata:
  name: fabric1
spec:
  nodes:
    leaf1: 
      provider: srlinux.nokia.com
      labels:
        inv.nephio.org/rack: rack1
        inv.nephio.org/rack-index: "0"
        inv.nephio.org/redundancy-group: rack1
    leaf2: 
      provider: srlinux.nokia.com
      labels:
        inv.nephio.org/rack: rack1
        inv.nephio.org/rack-index: "1"
        inv.nephio.org/redundancy-group: rack1
  links:
  - endpoints: 
    - { nodeName: leaf1, interfaceName: e1-30}
    - { nodeName: leaf2, interfaceName: e1-30}
    labels:
      nephio.org/purpose: infra
EOF
```

In the example above we interconnect 2 srlinux.nokia.com nodes using interface e1-30 between nodes lead1 and leaf2.

### metadata.name

the name of the topology

### spec.nodes

In a topology, nodes serve as the primary building blocks, defining the fundamental elements within the structure. Each node encompasses essential attributes, such as:

1. Provider: This attribute designates the provider responsible for implementing the node. The provider can be either virtual or physical, depending on the context of the topology.
2. NodeConfig: The NodeConfig refers to a NodeConfig Custom Resource Definition (CRD) that allows for provider-specific information to be included, such as the model, image, license, and parametersRef for additional vendor-specific details.
3. Labels: The labels attribute permits the inclusion of metadata in the form of key/value pairs. This feature aids in categorizing and organizing nodes within the topology.

When a node references a model, it automatically applies all the endpoints present in the model towards the Kubernetes APIservers,

### spec.links

A link defines the interconnection between two nodes by utilizing their respective endpoints. This crucial piece of information determines how the nodes in the topology are interconnected. In a virtual environment, this information can also be used as input to establish the connectivity between the respective nodes.

A link comprises of two endpoints, each consisting of a node and an interface name. These endpoints play a vital role in establishing the communication paths between the nodes, enabling effective data transfer and communication within the system.

A link has the capability to interconnect multiple topologies and is not limited to a single topology. Consequently, it allows for the construction of a multi-topology interconnect. This versatile feature enables the seamless integration of various topological configurations, facilitating more complex and flexible network setups.

## multi-topology

By default, the `RawTopology` CR (Custom Resource) name is utilized as the topology name. Consequently, a single topology represents the default usage of the raw topology. To facilitate the inclusion of multiple topologies within a single raw topology CR, the system allows specifying a topology under a node. This feature enables the support of multiple topologies. Both the Link and endpoints will inherit the topology from the node, enabling the construction of a multi-topology using the single `RawTopology` Custom Resource Definition (CRD).