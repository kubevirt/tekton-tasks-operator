# Tekton Tasks Operator

Tekton Tasks Operator is Go operator which takes care about 
deploying [Kubevirt tekton tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) 
and example pipelines.

## Installation
The `Tekton Tasks Operator` can run on both Kubernetes and OpenShift. However to be able to 
use all tasks, we recommend to use OpenShift (due to support for templates).

## Building

The Make will try to install kustomize, however if it is already installed it will not reinstall it.
In case of an error, make sure you are using at least v3 of kustomize, available here: https://kustomize.io/

To build the container image run:
```shell
make container-build
```

To upload the image to the default repository run:
```shell
make container-push
```
After the image is pushed to the repository,
manifests and the operator can be deployed using:
```shell
make deploy
```