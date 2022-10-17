#!/bin/bash

cp -L $KUBECONFIG /tmp/kubeconfig && export KUBECONFIG=/tmp/kubeconfig
export IMG=${CI_OPERATOR_IMG}

# Deploy resources
echo "Deploying resources"
./scripts/deploy-resources.sh

# SECRET
accessKeyId="/tmp/secrets/accessKeyId"
secretKey="/tmp/secrets/secretKey"

if test -f "$accessKeyId" && test -f "$secretKey"; then
  id=$(cat $accessKeyId | tr -d '\n' | base64)
  token=$(cat $secretKey | tr -d '\n' | base64 | tr -d ' \n')

  oc apply -n kubevirt -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: tekton-operator-container-disk-puller
  namespace: kubevirt
type: Opaque
data:
  accessKeyId: "${id}"
  secretKey: "${token}"
EOF
fi

function wait_until_exists() {
  timeout 10m bash <<- EOF
  until oc get $1; do
    sleep 5
  done
EOF
}

function wait_for_pipelinerun() {
  oc wait -n kubevirt --for=condition=Succeeded=True pipelinerun -l pipelinerun=$1-run --timeout=60m &
  success_pid=$!

  oc wait -n kubevirt --for=condition=Succeeded=False pipelinerun -l pipelinerun=$1-run --timeout=60m && exit 1 &
  failure_pid=$!

  wait -n $success_pid $failure_pid

  if (( $? == 0 )); then
    echo "Pipelinerun $1 succeeded"
  else
    echo "Pipelinerun $1 failed"
    exit 1
  fi
}

# Disable smart cloning - it does not work properly on azure clusters, when this issue gets fixed we can enable it again - https://issues.redhat.com/browse/CNV-21844
oc patch cdi cdi --type merge -p '{"spec":{"cloneStrategyOverride":"copy"}}'

echo "Creating datavolume with win10 iso"
oc apply -f "scripts/test-files/win10-dv.yaml"

echo "Waiting for pvc to be created"
wait_until_exists "pvc -n kubevirt iso-dv -o jsonpath='{.metadata.annotations.cdi\.kubevirt\.io/storage\.pod\.phase}'"
oc wait -n kubevirt pvc iso-dv --timeout=10m --for=jsonpath='{.metadata.annotations.cdi\.kubevirt\.io/storage\.pod\.phase}'='Succeeded'

echo "Create config map for http server"
oc apply -f "scripts/test-files/configmap.yaml"

echo "Deploying http-server to serve iso file to pipeline"
oc apply -f "scripts/test-files/http-server.yaml"

wait_until_exists "pods -n kubevirt -l app=http-server"

echo "Waiting for http server to be ready"
oc wait -n kubevirt --for=condition=Ready pod -l app=http-server --timeout=10m

echo "Deploy TTO and create sample"
make deploy

# Deploy tekton task sample
oc apply -f "config/samples/tektontasks_v1alpha1_tektontasks.yaml"
wait_until_exists "pipeline windows10-installer -n kubevirt" wait_until_exists "pipeline windows10-customize -n kubevirt"

# Run windows10-installer pipeline
echo "Running windows10-installer pipeline"
oc create -n kubevirt -f "scripts/test-files/windows10-installer-pipelinerun.yaml"
wait_until_exists "pipelinerun -n kubevirt -l pipelinerun=windows10-installer-run"

# Wait for pipeline to finish
echo "Waiting for pipeline to finish"
wait_for_pipelinerun "windows10-installer"

# Run windows10-customize pipeline
echo "Running windows10-customize pipeline"
oc create -n kubevirt -f "scripts/test-files/windows10-customize-pipelinerun.yaml"
wait_until_exists "pipelinerun -n kubevirt -l pipelinerun=windows10-customize-run"

# Wait for pipeline to finish
echo "Waiting for pipeline to finish"
wait_for_pipelinerun "windows10-customize"
