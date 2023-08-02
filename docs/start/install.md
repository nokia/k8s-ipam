# getting started

The resource backend is an application that runs within a kubernetes cluster. Through customer resources it enables a declarative management of resources, such as ip addresses, vlans, autonomous-systems, lag, esi, etc as well as inventory

## Pre-requisites

### Install a Kubernetest Cluster

Install a kubernetes cluster based on your preferences. Some examples are provided here for reference, but if you would use another kubernetes cluster flavor you can skip this step.

=== "kind cluster"

    Install the kind sw

    [install kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
    
    Create the kind cluster

    ```markdown
    kind create cluster
    ```
    
    Check if the cluster is running

    ```markdown
    kubectl get node
    ```
   
    Expected output or similar

    ```markdown
    kubectl get node
    NAME                 STATUS   ROLES           AGE   VERSION
    kind-control-plane   Ready    control-plane   8h    v1.27.3
    ```

### Install resource-backend

Install resource-backend using kpt

```
kpt pkg get --for-deployment "https://github.com/nokia/k8s-ipam.git/blueprint/resource-backend" resource-backend
kpt fn render resource-backend
kpt live init resource-backend
kpt live apply resource-backend
```

check if the resource-backend is running

```
kubectl get pods -n backend-system
```

a similar output is expected

NAME                                           READY   STATUS    RESTARTS   AGE
resource-backend-controller-5fd6976bdf-57knl   2/2     Running   0          6h13m
```


When all of this succeeded we can starrt provisioning inventory resources