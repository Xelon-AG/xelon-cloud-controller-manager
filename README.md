# Kubernetes Cloud Controller Manager for Xelon


## Overview

`xelon-cloud-controller-manager` (CCM) is the Kubernetes cloud controller manager implementation for Xelon. Read more
about cloud controller managers [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).
Running CCM allows you to leverage many of the cloud provider features offered by Xelon on your Kubernetes clusters.

The `xelon-cloud-controller-manager` provides a fully supported experience of Xelon features in your Kubernetes cluster:

- Node resources are assigned their respective Xelon instance hostnames, types and public/private IPs
- Xelon LoadBalancer Clusters are automatically deployed when a LoadBalancer service is deployed

> Note that this CCM is installed by default on [XKS](https://www.xelon.ch/products/kubernetes/) (Xelon Managed
> Kubernetes), you don't have to do it yourself.

## Contributing

We hope you'll get involved! Read our [Contributors' Guide](.github/CONTRIBUTING.md) for details.
