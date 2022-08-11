package tests

import (
	"strings"
	"time"

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
	Context("resource creation when DeployTektonTaskResources is set to false", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = false
			apiClient.Update(ctx, tto)
		})

		It("[test_id:TODO]operator should not create any cluster tasks", func() {
			liveTasks := &pipeline.ClusterTaskList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveTasks,
					client.MatchingLabels{
						tektontasks.TektonTasksVersionLabel: operands.TektonTasksVersion,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveTasks.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "tasks should not exists")
		})

		It("[test_id:TODO]operator should not create any service accounts", func() {
			liveSA := &v1.ServiceAccountList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveSA,
					client.MatchingLabels{
						tektontasks.TektonTasksVersionLabel: operands.TektonTasksVersion,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveSA.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "service accounts should not exists")
		})

		It("[test_id:TODO]operator should not create any cluster role", func() {
			liveCR := &rbac.ClusterRoleList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveCR,
					client.MatchingLabels{
						tektontasks.TektonTasksVersionLabel: operands.TektonTasksVersion,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveCR.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "cluster role should not exists")
		})

		It("[test_id:TODO]operator should not create role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						tektontasks.TektonTasksVersionLabel: operands.TektonTasksVersion,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "role bindings should not exists")
		})

	})
	Context("resource creation", func() {
		BeforeEach(func() {
			tto := strategy.GetTTO()
			tto.Spec.FeatureGates.DeployTektonTaskResources = true
			createOrUpdateTekton(tto)
			waitUntilDeployed()
		})

		It("[test_id:TODO]operator should create only allowed tekton-tasks with correct labels", func() {
			liveTasks := &pipeline.ClusterTaskList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveTasks,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveTasks.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

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
			Eventually(func() bool {
				err := apiClient.List(ctx, liveSA,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveSA.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())
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
			clusterRoleName := "windows10-pipelines"
			Eventually(func() bool {
				err := apiClient.List(ctx, liveCR,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveCR.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())
			for _, cr := range liveCR.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(cr.Name, "-task")]; !ok {
					if ok = cr.Name != clusterRoleName; ok {
						Expect(ok).To(BeTrue(), "only allowed cluster role is deployed - "+cr.Name)
					}
				}

				if cr.Name == clusterRoleName {
					Expect(cr.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonPipelines)), "component label should equal")
				} else {
					Expect(cr.Labels[common.AppKubernetesComponentLabel]).To(Equal(string(common.AppComponentTektonTasks)), "component label should equal")
				}
				Expect(cr.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
			}
		})

		It("[test_id:TODO]operator should create role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) > 0
			}, tenSecondTimeout, time.Second).Should(BeTrue())

			for _, rb := range liveRB.Items {
				if _, ok := tektontasks.AllowedTasks[strings.TrimSuffix(rb.Name, "-task")]; !ok {
					Expect(ok).To(BeTrue(), "only allowed role binding is deployed - "+rb.Name)
				}
				Expect(rb.Labels[common.AppKubernetesManagedByLabel]).To(Equal(common.AppKubernetesManagedByValue), "managed by label should equal")
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

		It("[test_id:TODO]operator should delete tekton-tasks", func() {
			liveTasks := &pipeline.ClusterTaskList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveTasks,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveTasks.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no cluster tasks left")
		})

		It("[test_id:TODO]operator should delete service accounts", func() {
			liveSA := &v1.ServiceAccountList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveSA,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveSA.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no service accounts left")
		})

		It("[test_id:TODO]operator should delete cluster role", func() {
			liveCR := &rbac.ClusterRoleList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveCR,
					client.MatchingLabels{
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveCR.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no cluster roles left")

		})

		It("[test_id:TODO]operator should delete role bindings", func() {
			liveRB := &rbac.RoleBindingList{}
			Eventually(func() bool {
				err := apiClient.List(ctx, liveRB,
					client.MatchingLabels{
						common.AppKubernetesComponentLabel: string(common.AppComponentTektonTasks),
						common.AppKubernetesManagedByLabel: common.AppKubernetesManagedByValue,
					},
				)
				Expect(err).ToNot(HaveOccurred())
				return len(liveRB.Items) == 0
			}, tenSecondTimeout, time.Second).Should(BeTrue(), "there should be no role bindings left")
		})
	})
})
