#!/usr/bin/env bash

set -ex

if oc get namespace tekton-pipelines > /dev/null 2>&1; then
  exit 0
fi

KUBEVIRT_VERSION=$(curl -s https://github.com/kubevirt/kubevirt/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
CDI_VERSION=$(curl -s https://github.com/kubevirt/containerized-data-importer/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")

TEKTON_VERSION=$(curl -s https://api.github.com/repos/tektoncd/operator/releases | \
            jq '.[] | select(.prerelease==false) | .tag_name' | sort -V | tail -n1 | tr -d '"')

if oc get templates > /dev/null 2>&1; then
  # Prepare Tekton Pipelines
  oc adm policy add-scc-to-user anyuid -z tekton-pipelines-controller
  oc adm policy add-scc-to-user anyuid -z tekton-pipelines-webhook
fi

# Deploy Tekton Pipelines
oc apply -f "https://github.com/tektoncd/operator/releases/download/${TEKTON_VERSION}/openshift-release.yaml"

# Deploy Kubevirt
oc apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

oc apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"

oc patch kubevirt kubevirt -n kubevirt --type merge -p '{"spec":{"configuration":{"developerConfiguration":{"featureGates": ["DataVolumes"]}}}}'

# Deploy Storage
oc apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml"

oc apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml"

# wait for tekton pipelines
oc rollout status -n openshift-pipelines deployment/tekton-pipelines-controller --timeout 10m
oc rollout status -n openshift-pipelines deployment/tekton-pipelines-webhook --timeout 10m

# Wait for kubevirt to be available
oc rollout status -n cdi deployment/cdi-operator --timeout 10m
oc wait -n kubevirt kv kubevirt --for condition=Available --timeout 10m
