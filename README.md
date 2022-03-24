# Tekton Tasks Operator

Tekton Tasks Operator is Go operator which takes care about 
deploying [Kubevirt tekton tasks](https://github.com/kubevirt/kubevirt-tekton-tasks) 
and example pipelines.

## Prerequisites
- [Tekton](https://tekton.dev/)
- [KubeVirt](https://kubevirt.io/)
- [CDI](https://github.com/kubevirt/containerized-data-importer)

To install all prerequisities, just run:
```shell
./scripts/deploy-resources.sh
```

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

## Deploy
After the image is pushed to the repository,
manifests and the operator can be deployed using:
```shell
make deploy
```
And deploy TTO CR
```shell
oc create -f config/samples/tektontasks_v1alpha1_tektontasks.yaml
```
Tekton tasks operator does not deploy tekton tasks and example pipelines by default.
User has to update `spec.featureGates.deployTektonTaskResources` in TTO CR to true to trigger reconciliation.

## Testing

### e2e tests
```shell
make container-build
```

### unit tests
```shell
make unittest
```