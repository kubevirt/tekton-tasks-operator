package tests

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

var _ = Describe("Tekton-pipelines", func() {
	Context("resource creation", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = true
			createOrUpdateTekton(tto)
			waitUntilDeployed()
		})

		It("[test_id:TODO]operator should create pipelines in correct namespace", func() {
			livePipelines := &pipeline.PipelineList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, livePipelines,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(livePipelines.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			for _, pipeline := range livePipelines.Items {
				Expect(pipeline.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
				Expect(pipeline.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})
	})

	Context("resource deletion when CR is deleted", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			apiClient.Delete(ctx, tto)
		})

		AfterEach(func() {
			strategy.CreateTTOIfNeeded()
		})

		It("[test_id:TODO]operator should delete pipelines", func() {
			livePipelines := &pipeline.PipelineList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, livePipelines,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(livePipelines.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no pipelines left")
		})
	})
})
