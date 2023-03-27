/*
Copyright The Kubernetes NMState Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package environment

import (
	"os"
	"time"

	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
	"github.com/pkg/errors"
)

const (
	OperatorVersionKey        = "OPERATOR_VERSION"
	OperatorNamespaceKey      = "OPERATOR_NAMESPACE"
	CleanupVMImageKey         = "CLEANUP_VM_IMG"
	CopyTemplateImageKey      = "COPY_TEMPLATE_IMG"
	ModifyDataObjectImageKey  = "MODIFY_DATA_OBJECT_IMG"
	CreateVMImageKey          = "CREATE_VM_IMG"
	DiskVirtCustomizeImageKey = "DISK_VIRT_CUSTOMIZE_IMG"
	DiskVirtSysprepImageKey   = "DISK_VIRT_SYSPREP_IMG"
	ModifyVMTemplateImageKey  = "MODIFY_VM_TEMPLATE_IMG"
	WaitForVMISTatusImageKey  = "WAIT_FOR_VMI_STATUS_IMG"
	VirtioImageKey            = "VIRTIO_IMG"
	GenerateSSHKeysImageKey   = "GENERATE_SSH_KEYS_IMG"

	DefaultWaitForVMIStatusIMG  = "quay.io/kubevirt/tekton-task-wait-for-vmi-status:" + operands.TektonTasksVersion
	DeafultModifyVMTemplateIMG  = "quay.io/kubevirt/tekton-task-modify-vm-template:" + operands.TektonTasksVersion
	DeafultDiskVirtSysprepIMG   = "quay.io/kubevirt/tekton-task-disk-virt-sysprep:" + operands.TektonTasksVersion
	DeafultDiskVirtCustomizeIMG = "quay.io/kubevirt/tekton-task-disk-virt-customize:" + operands.TektonTasksVersion
	DeafultCreateVMIMG          = "quay.io/kubevirt/tekton-task-create-vm:" + operands.TektonTasksVersion
	DeafultModifyDataObjectIMG  = "quay.io/kubevirt/tekton-task-modify-data-object:" + operands.TektonTasksVersion
	DeafultCopyTemplateIMG      = "quay.io/kubevirt/tekton-task-copy-template:" + operands.TektonTasksVersion
	DeafultCleanupVMIMG         = "quay.io/kubevirt/tekton-task-execute-in-vm:" + operands.TektonTasksVersion
	GenerateSSHKeysIMG          = "quay.io/kubevirt/tekton-task-generate-ssh-keys:" + operands.TektonTasksVersion
	DefaultVirtioIMG            = "quay.io/kubevirt/virtio-container-disk:v0.59.0"

	defaultOperatorVersion = "devel"
)

// GetSSHKeysStatusImage returns generate-ssh-keys task image url
func GetSSHKeysStatusImage() string {
	return EnvOrDefault(GenerateSSHKeysImageKey, GenerateSSHKeysIMG)
}

// GetWaitForVMIStatusImage returns wait-for-vmi-status task image url
func GetWaitForVMIStatusImage() string {
	return EnvOrDefault(WaitForVMISTatusImageKey, DefaultWaitForVMIStatusIMG)
}

// GetModifyVMTemplateImage returns modify-vm-template task image url
func GetModifyVMTemplateImage() string {
	return EnvOrDefault(ModifyVMTemplateImageKey, DeafultModifyVMTemplateIMG)
}

// GetDiskVirtSysprepImage returns disk-virt-sysprep task image url
func GetDiskVirtSysprepImage() string {
	return EnvOrDefault(DiskVirtSysprepImageKey, DeafultDiskVirtSysprepIMG)
}

// GetDiskVirtCustomizeImage returns disk-virt-customize task image url
func GetDiskVirtCustomizeImage() string {
	return EnvOrDefault(DiskVirtCustomizeImageKey, DeafultDiskVirtCustomizeIMG)
}

// GetCreateVMImage returns create-vm-from-manifest task image url
func GetCreateVMImage() string {
	return EnvOrDefault(CreateVMImageKey, DeafultCreateVMIMG)
}

// GetModifyDataObjectImage returns modify-data-object task image url
func GetModifyDataObjectImage() string {
	return EnvOrDefault(ModifyDataObjectImageKey, DeafultModifyDataObjectIMG)
}

// GetCopyTemplatemage returns copy-template task image url
func GetCopyTemplateImage() string {
	return EnvOrDefault(CopyTemplateImageKey, DeafultCopyTemplateIMG)
}

// GetCleanupVMImage returns cleanup-vm task image url
func GetCleanupVMImage() string {
	return EnvOrDefault(CleanupVMImageKey, DeafultCleanupVMIMG)
}

// GetVirtioImage returns virtio image url
func GetVirtioImage() string {
	return EnvOrDefault(VirtioImageKey, DefaultVirtioIMG)
}

// GetModifyVMTemplateImage returns modify-vm-template task image url
func GetOperatorNamespace() string {
	return EnvOrDefault(OperatorNamespaceKey, "kubevirt")
}

func LookupAsDuration(varName string) (time.Duration, error) {
	duration := time.Duration(0)
	varValue, ok := os.LookupEnv(varName)
	if !ok {
		return duration, errors.Errorf("Failed to load %s from environment", varName)
	}

	duration, err := time.ParseDuration(varValue)
	if err != nil {
		return duration, errors.Wrapf(err, "Failed to convert %s value to time.Duration", varName)
	}
	return duration, nil
}

func GetOperatorVersion() string {
	return EnvOrDefault(OperatorVersionKey, defaultOperatorVersion)
}

func EnvOrDefault(envName string, defVal string) string {
	val := os.Getenv(envName)
	if val == "" {
		return defVal
	}
	return val
}
