package tests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

var _ = Describe("Tekton-pipelines", func() {
	Context("resource creation", func() {
		It("[test_id:TODO]operator should create pipelines in correct namespace", func() {
			livePipelines := &pipeline.PipelineList{}
			err := apiClient.List(ctx, livePipelines, client.MatchingLabels{
				common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(len(livePipelines.Items) > 0).To(BeTrue(), "pipelines has to exists")
			for _, pipeline := range livePipelines.Items {
				Expect(pipeline.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
				Expect(pipeline.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})
	})
})
