package common

import (
	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AppComponent string

func (a AppComponent) String() string {
	return string(a)
}

const (
	AppKubernetesNameLabel      = "app.kubernetes.io/name"
	AppKubernetesPartOfLabel    = "app.kubernetes.io/part-of"
	AppKubernetesVersionLabel   = "app.kubernetes.io/version"
	AppKubernetesManagedByLabel = "app.kubernetes.io/managed-by"
	AppKubernetesComponentLabel = "app.kubernetes.io/component"

	AppComponentTektonTasks     AppComponent = "tektonTasks"
	AppComponentTektonPipelines AppComponent = "tektonPipelines"
	AppKubernetesManagedByValue              = "tekton-tasks-operator"
)

func AddAppLabels(requestInstance *tekton.TektonTasks, name string, component AppComponent, obj client.Object) client.Object {
	labels := getOrCreateLabels(obj)
	addInstanceLabels(requestInstance, labels)

	labels[AppKubernetesNameLabel] = name
	labels[AppKubernetesComponentLabel] = component.String()
	labels[AppKubernetesManagedByLabel] = AppKubernetesManagedByValue

	return obj
}

func getOrCreateLabels(obj client.Object) map[string]string {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
		obj.SetLabels(labels)
	}
	return labels
}

func addInstanceLabels(requestInstance *tekton.TektonTasks, to map[string]string) {
	if requestInstance.Labels == nil {
		return
	}

	copyLabel(requestInstance.Labels, to, AppKubernetesPartOfLabel)
	copyLabel(requestInstance.Labels, to, AppKubernetesVersionLabel)
}

func copyLabel(from, to map[string]string, key string) {
	to[key] = from[key]
}
