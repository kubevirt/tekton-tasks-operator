package main

import (
	"testing"

	"github.com/blang/semver/v4"
	"github.com/kubevirt/tekton-tasks-operator/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("csv generator", func() {
	flags := generatorFlags{
		dumpCRDs:           true,
		removeCerts:        false,
		csvVersion:         "9.9.9",
		operatorNamespace:  "testOperatorNamespace",
		pipelinesNamespace: "testPipelineNamespace",
		operatorVersion:    "testOperatorVersion",

		waitForVMIStatusImage:  "testWaitImage",
		modifyVMTemplateImage:  "testModifyImage",
		diskVirtSysprepImage:   "testSysprepImage",
		diskVirtCustomizeImage: "testCustomizeImage",
		createVMImage:          "testCreateVMImage",
		createDatavolumeImage:  "testDatavolumeImage",
		copyTemplateImage:      "testCopyTemplateImage",
		cleanupVMImage:         "testCleanupImage",
	}
	envValues := []v1.EnvVar{
		{Name: environment.OperatorVersionKey},
		{Name: environment.CleanupVMImageKey},
		{Name: environment.CopyTemplateImageKey},
		{Name: environment.CreateDatavolumeImageKey},
		{Name: environment.CreateVMImageKey},
		{Name: environment.DiskVirtCustomizeImageKey},
		{Name: environment.DiskVirtSysprepImageKey},
		{Name: environment.ModifyVMTemplateImageKey},
		{Name: environment.WaitForVMISTatusImageKey},
	}

	csv := csvv1.ClusterServiceVersion{
		Spec: csvv1.ClusterServiceVersionSpec{
			InstallStrategy: csvv1.NamedInstallStrategy{
				StrategySpec: csvv1.StrategyDetailsDeployment{
					DeploymentSpecs: []csvv1.StrategyDeploymentSpec{
						{
							Spec: appsv1.DeploymentSpec{
								Template: v1.PodTemplateSpec{
									Spec: v1.PodSpec{
										Containers: []v1.Container{
											{
												Name: "manager",
												Env:  envValues,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	It("should update csv", func() {
		err := replaceVariables(flags, &csv)
		Expect(err).To(BeNil())
		//test csv name
		Expect(csv.Name).To(Equal("tekton-tasks-operator.v9.9.9"))
		//test csv version
		v, err := semver.New(flags.csvVersion)
		Expect(err).To(BeNil())
		Expect(csv.Spec.Version).To(Equal(version.OperatorVersion{Version: *v}))

		for _, container := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec.Containers {
			if container.Name == "manager" {
				Expect(container.Image).To(Equal(flags.operatorImage))

				for _, envVariable := range container.Env {
					if envVariable.Name == environment.OperatorVersionKey {
						Expect(envVariable.Value).To(Equal(flags.operatorVersion))
					}
					if envVariable.Name == environment.CleanupVMImageKey {
						Expect(envVariable.Value).To(Equal(flags.cleanupVMImage))
					}
					if envVariable.Name == environment.CopyTemplateImageKey {
						Expect(envVariable.Value).To(Equal(flags.copyTemplateImage))
					}
					if envVariable.Name == environment.CreateDatavolumeImageKey {
						Expect(envVariable.Value).To(Equal(flags.createDatavolumeImage))
					}
					if envVariable.Name == environment.CreateVMImageKey {
						Expect(envVariable.Value).To(Equal(flags.createVMImage))
					}
					if envVariable.Name == environment.DiskVirtCustomizeImageKey {
						Expect(envVariable.Value).To(Equal(flags.diskVirtCustomizeImage))
					}
					if envVariable.Name == environment.DiskVirtSysprepImageKey {
						Expect(envVariable.Value).To(Equal(flags.diskVirtSysprepImage))
					}
					if envVariable.Name == environment.ModifyVMTemplateImageKey {
						Expect(envVariable.Value).To(Equal(flags.modifyVMTemplateImage))
					}
					if envVariable.Name == environment.WaitForVMISTatusImageKey {
						Expect(envVariable.Value).To(Equal(flags.waitForVMIStatusImage))
					}
				}
				break
			}
		}
	})
})

func TestCsvGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Csv generator Suite")
}
