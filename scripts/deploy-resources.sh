#!/usr/bin/env bash

set -ex

if oc get namespace tekton-pipelines > /dev/null 2>&1; then
  exit 0
fi

KUBEVIRT_VERSION="v0.58.0"
#$(curl -s https://api.github.com/repos/kubevirt/kubevirt/releases | \
#           jq '.[] | select(.prerelease==false) | .tag_name' | sort -V | tail -n1 | tr -d '"')

CDI_VERSION=$(curl -s https://api.github.com/repos/kubevirt/containerized-data-importer/releases | \
            jq '.[] | select(.prerelease==false) | .tag_name' | sort -V | tail -n1 | tr -d '"')

TEKTON_VERSION=$(curl -s https://api.github.com/repos/tektoncd/operator/releases | \
            jq '.[] | select(.prerelease==false) | .tag_name' | sort -V | tail -n1 | tr -d '"')

SSP_OPERATOR_VERSION=$(curl -s  https://api.github.com/repos/kubevirt/ssp-operator/releases | \
            jq '.[] | select(.prerelease==false) | .tag_name' | sort -V | tail -n1 | tr -d '"')

# Deploy Tekton Pipelines
oc apply -f "https://github.com/tektoncd/operator/releases/download/${TEKTON_VERSION}/openshift-release.yaml"

# Deploy Kubevirt
oc apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

oc apply -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"

oc patch kubevirt kubevirt -n kubevirt --type merge -p '{"spec":{"configuration":{"developerConfiguration":{"featureGates": ["DataVolumes"]}}}}'

# Deploy Storage
oc apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml"

oc apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml"

# Deploy SSP
oc apply -f "https://github.com/kubevirt/ssp-operator/releases/download/${SSP_OPERATOR_VERSION}/ssp-operator.yaml"
oc wait -n kubevirt deployment ssp-operator --for condition=Available --timeout 10m

oc apply -f "scripts/ssp-sample.yaml"
oc wait -n kubevirt ssp ssp-sample --for condition=Available --timeout 10m

# wait for tekton pipelines
oc rollout status -n openshift-operators deployment/openshift-pipelines-operator --timeout 10m

# wait until clustertasks tekton CRD is properly deployed
timeout 10m bash <<- EOF
  until oc get crd clustertasks.tekton.dev; do
    sleep 5
  done
EOF

# wait until tekton pipelines webhook is created
timeout 10m bash <<- EOF
  until oc get deployment tekton-pipelines-webhook -n openshift-pipelines; do
    sleep 5
  done
EOF

# wait until tekton pipelines webhook is online
oc wait -n openshift-pipelines deployment tekton-pipelines-webhook --for condition=Available --timeout 10m

# Wait for kubevirt to be available
oc rollout status -n cdi deployment/cdi-operator --timeout 10m
oc wait -n kubevirt kv kubevirt --for condition=Available --timeout 10m
