package tests

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
	tektontasks "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-tasks"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Tekton-tasks", func() {
	Context("resource creation", func() {
		It("[test_id:TODO]operator should create only allowed tekton-tasks with correct labels", func() {
			liveTasks := &pipeline.ClusterTaskList{}
			err := apiClient.List(ctx, liveTasks,
				client.MatchingLabels{
					tektontasks.TektonTasksVersionLabel: operands.TektonTasksVersion,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(liveTasks.Items) > 0).To(BeTrue(), "tasks has to exists")
			for _, task := range liveTasks.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(task.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed task is deployed")
				}
				Expect(task.Labels[tektontasks.TektonTasksVersionLabel]).To(Equal(operands.TektonTasksVersion), "version label should equal")
				Expect(task.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				Expect(task.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create service accounts", func() {
			liveSA := &v1.ServiceAccountList{}
			err := apiClient.List(ctx, liveSA,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(liveSA.Items) > 0).To(BeTrue(), "service accounts have to exists")
			for _, sa := range liveSA.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(sa.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed service account is deployed - "+sa.Name)
				}
				Expect(sa.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				Expect(sa.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create cluster role", func() {
			liveCR := &rbac.ClusterRoleList{}
			err := apiClient.List(ctx, liveCR,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(liveCR.Items) > 0).To(BeTrue(), "cluster role have to exists")
			for _, cr := range liveCR.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(cr.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed cluster role is deployed - "+cr.Name)
				}
				Expect(cr.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				Expect(cr.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			err := apiClient.List(ctx, liveRB,
				client.MatchingLabels{
					common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
				},
			)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(liveRB.Items) > 0).To(BeTrue(), " role bindings have to exists")
			for _, rb := range liveRB.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(rb.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed role binding is deployed - "+rb.Name)
				}
				Expect(rb.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				Expect(rb.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})
	})
})
