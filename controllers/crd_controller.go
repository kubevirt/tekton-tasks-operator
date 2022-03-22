package controllers

import (
	"context"
	"errors"
	"sync"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"github.com/kubevirt/tekton-tasks-operator/controllers/finishable"
	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	"github.com/kubevirt/tekton-tasks-operator/pkg/environment"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

func CreateCrdController(mgr controllerruntime.Manager, requiredCrds []string) (finishable.Controller, error) {
	crds := make(map[string]bool, len(requiredCrds))
	for _, crd := range requiredCrds {
		crds[crd] = false
	}

	reconciler := &waitForCrds{
		client: mgr.GetClient(),
		crds:   crds,
	}

	initCtrl, err := finishable.NewController("init-controller", mgr, reconciler)
	if err != nil {
		return nil, err
	}

	err = initCtrl.Watch(&source.Kind{Type: &extv1.CustomResourceDefinition{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return initCtrl, nil
}

type waitForCrds struct {
	client client.Client

	lock sync.RWMutex
	crds map[string]bool
}

var _ finishable.Reconciler = &waitForCrds{}

func (w *waitForCrds) Reconcile(ctx context.Context, request reconcile.Request) (finishable.Result, error) {
	err := setConditions(w.client)
	if err != nil {
		return finishable.Result{}, err
	}

	crdExists := true
	crd := &extv1.CustomResourceDefinition{}
	err = w.client.Get(ctx, request.NamespacedName, crd)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return finishable.Result{}, err
		}
		crdExists = false
	}

	// If CRD is being deleted, we treat it as not existing.
	if !crd.GetDeletionTimestamp().IsZero() {
		crdExists = false
	}

	key := request.NamespacedName.Name
	if w.isCrdRequired(key) {
		w.setCrdExists(key, crdExists)
	}

	return finishable.Result{Finished: w.allCrdsExist()}, nil
}

func (w *waitForCrds) isCrdRequired(key string) bool {
	w.lock.RLock()
	defer w.lock.RUnlock()

	_, exists := w.crds[key]
	return exists
}

func (w *waitForCrds) setCrdExists(key string, val bool) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.crds[key] = val
}

func (w *waitForCrds) allCrdsExist() bool {
	w.lock.RLock()
	defer w.lock.RUnlock()

	allExist := true
	for _, exists := range w.crds {
		allExist = allExist && exists
	}
	return allExist
}

func setConditions(cli client.Client) error {
	operatorNamespace := environment.GetOperatorNamespace()
	instances := &tekton.TektonTasksList{}
	err := cli.List(context.TODO(), instances, client.InNamespace(operatorNamespace))
	if err != nil {
		return err
	}

	if len(instances.Items) == 0 {
		return nil
	}

	if len(instances.Items) > 1 {
		return errors.New("multiple TektonTasks CRs in single namespace")
	}

	err = updateStatus(&common.Request{Instance: &instances.Items[0], Client: cli, Context: context.TODO()}, []common.ReconcileResult{})
	if err != nil {
		return err
	}
	return nil
}
