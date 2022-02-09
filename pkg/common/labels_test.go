package common

import (
	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("AddAppLabels", func() {
	var (
		request Request
	)

	BeforeEach(func() {
		request = Request{
			Instance: &tekton.TektonTasks{
				TypeMeta: metav1.TypeMeta{
					Kind:       tektonResourceKind,
					APIVersion: tekton.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-tekton-tasks",
					Labels: map[string]string{
						AppKubernetesPartOfLabel:  "tests",
						AppKubernetesVersionLabel: "v0.0.0-tests",
					},
				},
			},
		}
	})

	When("Tekton CR has app labels", func() {
		It("adds app labels from request", func() {
			obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

			labels := obj.GetLabels()
			Expect(labels[AppKubernetesPartOfLabel]).To(Equal("tests"))
			Expect(labels[AppKubernetesVersionLabel]).To(Equal("v0.0.0-tests"))
		})
	})
	When("Tekton CR does not have app labels", func() {
		It("does not add app labels on nil", func() {
			request.Instance.Labels = nil
			obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

			labels := obj.GetLabels()
			Expect(labels[AppKubernetesPartOfLabel]).To(Equal(""))
			Expect(labels[AppKubernetesVersionLabel]).To(Equal(""))
		})
		It("does not add app labels empty map", func() {
			request.Instance.Labels = map[string]string{}
			obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

			labels := obj.GetLabels()
			Expect(labels[AppKubernetesPartOfLabel]).To(Equal(""))
			Expect(labels[AppKubernetesVersionLabel]).To(Equal(""))
		})
	})

	It("adds dynamic app labels", func() {
		obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

		labels := obj.GetLabels()
		Expect(labels[AppKubernetesComponentLabel]).To(Equal("testing"))
		Expect(labels[AppKubernetesNameLabel]).To(Equal("test"))
	})

	It("adds managed-by label", func() {
		obj := AddAppLabels(request.Instance, "test", AppComponent("testing"), &v1.ConfigMap{})

		labels := obj.GetLabels()
		Expect(labels[AppKubernetesManagedByLabel]).To(Equal(AppKubernetesManagedByValue))
	})
})
