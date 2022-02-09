package common

import (
	"context"

	"github.com/go-logr/logr"
	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	osconfv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Request struct {
	reconcile.Request
	Client         client.Client
	UncachedReader client.Reader
	Context        context.Context
	Logger         logr.Logger
	Instance       *tekton.TektonTasks
	VersionCache   VersionCache
	TopologyMode   osconfv1.TopologyMode
}
