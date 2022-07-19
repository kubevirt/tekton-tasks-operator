#!/bin/bash

cp -L $KUBECONFIG /tmp/kubeconfig && export KUBECONFIG=/tmp/kubeconfig
export IMG=${CI_OPERATOR_IMG}

winImageDownloadURL="http://http-server/downloads/disk.img"

# Deploy resources
echo "Deploying resources"
./scripts/deploy-resources.sh

# SECRET
key="/tmp/secrets/accessKeyId"
token="/tmp/secrets/secretKey"

if test -f "$key" && test -f "$token"; then
  id=$(cat $key | tr -d '\n' | base64)
  token=$(cat $token | tr -d '\n' | base64 | tr -d ' \n')

  oc apply -n default -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: tekton-operator-container-disk-puller
  namespace: default
type: Opaque
data:
  accessKeyId: "${id}"
  secretKey: "${token}"
EOF
fi

echo "Creating datavolume with win10 iso"
oc apply -f "scripts/test-files/win10-dv.yaml"
echo "Waiting for datavolume to be created"
oc wait -n default datavolume iso-dv --timeout=10m --for=jsonpath='{.status.phase}'='Succeeded'

echo "Deploying http-server to serve iso file to pipeline"
oc apply -f "scripts/test-files/http-server.yaml"

timeout 10m bash <<- EOF
  until oc get pods -n default | grep http-server-; do
    sleep 5
  done
EOF

POD_NAME=$(oc get pods -n default -o=name | awk '{if ($1 ~ "http-server-") print $0}')
echo "Waiting for http server to be ready"
oc wait -n default --for=condition=ready ${POD_NAME} --timeout=10m
# Edit permissions for nginx folder
oc exec -n default -i $POD_NAME -- sh -c "chmod -R 755 /downloads"

echo "Deploy TTO and create sample"
make deploy

# Deploy tekton task sample
oc apply -f "config/samples/tektontasks_v1alpha1_tektontasks.yaml"

# Wait until tekton task is created and jsonpath .status.phase exists
timeout 10m bash <<- EOF
  while [[ -z "$(oc get tektontasks -n kubevirt tektontasks-sample -o=jsonpath="{.status.phase}")" ]]; 
  do
    sleep 5
  done
EOF

# Wait until tektontasks are deployed
oc wait tektontasks tektontasks-sample -n kubevirt --for=jsonpath='{.status.phase}'='Deployed' --timeout=10m

timeout 10m bash <<- EOF
  until oc get pipeline windows10-installer -n kubevirt; do
    sleep 5
  done
EOF

timeout 10m bash <<- EOF
  until oc get pipeline windows10-customize -n kubevirt; do
    sleep 5
  done
EOF

# Run windows10-installer pipeline
echo "Running windows10-installer pipeline"
oc create -n kubevirt -f - <<EOF
apiVersion: tekton.dev/v1alpha1
kind: PipelineRun
metadata:
  generateName: windows10-installer-run-
spec:
  params:
  - name: winImageDownloadURL
    value: "${winImageDownloadURL}"
  pipelineRef:
    name: windows10-installer
  taskRunSpecs:
    - pipelineTaskName: copy-template
      taskServiceAccountName: copy-template-task
    - pipelineTaskName: modify-vm-template
      taskServiceAccountName: modify-vm-template-task
    - pipelineTaskName: create-vm-from-template
      taskServiceAccountName: create-vm-from-template-task
    - pipelineTaskName: wait-for-vmi-status
      taskServiceAccountName: wait-for-vmi-status-task
    - pipelineTaskName: create-base-dv
      taskServiceAccountName: create-data-object-task
    - pipelineTaskName: cleanup-vm
      taskServiceAccountName: cleanup-vm-task
status: {}
EOF

timeout 10m bash <<- EOF
  until oc get pipelinerun -n kubevirt | grep windows10-installer-run; do
    sleep 5
  done
EOF

# Wait for pipeline to finish
WINDOWS_INSTALLER=$(oc get pipelinerun -o=name -n kubevirt | awk '{if ($1 ~ "windows10-installer-") print $0}')
echo "Waiting for pipeline to finish"
oc wait -n kubevirt --for=condition=Succeeded --timeout=60m ${WINDOWS_INSTALLER}

# Run windows10-customize pipeline
echo "Running windows10-customize pipeline"
oc create -n kubevirt -f - <<EOF
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  generateName: windows10-customize-run-
spec:
  params:
    - name: allowReplaceGoldenTemplate
      value: true
    - name: allowReplaceCustomizationTemplate
      value: true
  pipelineRef:
    name: windows10-customize
  taskRunSpecs:
    - pipelineTaskName: copy-template-customize
      taskServiceAccountName: copy-template-task
    - pipelineTaskName: modify-vm-template-customize
      taskServiceAccountName: modify-vm-template-task
    - pipelineTaskName: create-vm-from-template
      taskServiceAccountName: create-vm-from-template-task
    - pipelineTaskName: wait-for-vmi-status
      taskServiceAccountName: wait-for-vmi-status-task
    - pipelineTaskName: create-base-dv
      taskServiceAccountName: create-data-object-task
    - pipelineTaskName: cleanup-vm
      taskServiceAccountName: cleanup-vm-task
    - pipelineTaskName: copy-template-golden
      taskServiceAccountName: copy-template-task
    - pipelineTaskName: modify-vm-template-golden
      taskServiceAccountName: modify-vm-template-task
status: {}
EOF

timeout 10m bash <<- EOF
  until oc get pipelinerun -n kubevirt | grep windows10-customize-run; do
    sleep 5
  done
EOF

# Wait for pipeline to finish
WINDOWS10_CUSTOMIZE=$(oc get pipelinerun -o=name -n kubevirt | awk '{if ($1 ~ "windows10-customize-") print $0}')
echo "Waiting for pipeline to finish"
oc wait -n kubevirt --for=condition=Succeeded --timeout=60m ${WINDOWS10_CUSTOMIZE}
