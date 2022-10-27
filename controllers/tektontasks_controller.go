/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	libhandler "github.com/operator-framework/operator-lib/handler"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	"github.com/kubevirt/tekton-tasks-operator/pkg/environment"
	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
)

const (
	finalizerName    = "tekton-tasks.kubevirt.io/finalizer"
	oldFinalizerName = "finalize.tekton-tasks.kubevirt.io"
)

// tektonTasksReconciler reconciles a TektonTasks object
type tektonTasksReconciler struct {
	client              client.Client
	log                 logr.Logger
	operands            []operands.Operand
	subresourceCache    common.VersionCache
	lastTektonTasksSpec tekton.TektonTasksSpec
}

func NewTektonReconciler(client client.Client, uncachedReader client.Reader, operands []operands.Operand) *tektonTasksReconciler {
	return &tektonTasksReconciler{
		client:           client,
		subresourceCache: common.VersionCache{},
		operands:         operands,
		log:              ctrl.Log.WithName("controllers").WithName("TektonTasksOperator"),
	}
}

//+kubebuilder:rbac:groups=tektontasks.kubevirt.io,resources=tektontasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=tektontasks.kubevirt.io,resources=tektontasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=tektontasks.kubevirt.io,resources=tektontasks/finalizers,verbs=update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *tektonTasksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	defer func() {
		if err != nil {
			common.TektonOperatorReconcilingProperly.Set(0)
		}
	}()
	reqLogger := r.log.WithValues("tekton-task-operator", req.NamespacedName)
	reqLogger.V(1).Info("Starting reconciliation...")

	instance := &tekton.TektonTasks{}
	err = r.client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	r.clearCacheIfNeeded(instance)

	tektonRequest := &common.Request{
		Request:      req,
		Client:       r.client,
		Context:      ctx,
		Instance:     instance,
		Logger:       reqLogger,
		VersionCache: r.subresourceCache,
	}

	if !isInitialized(tektonRequest.Instance) {
		err := initialize(tektonRequest)
		return handleError(tektonRequest, err)
	}

	if updated, err := updateTektonTasks(tektonRequest); updated || (err != nil) {
		// tekton was updated, and the update will trigger reconciliation again.
		return handleError(tektonRequest, err)
	}

	if isBeingDeleted(tektonRequest.Instance) {
		err := r.cleanup(tektonRequest)
		if err != nil {
			return ctrl.Result{}, err
		}
		r.clearCache()
		return ctrl.Result{}, nil
	}

	tektonTaskList, err := r.getCRList(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	if len(tektonTaskList.Items) > 1 {
		return ctrl.Result{}, r.setStatusMultipleCRs(ctx, tektonTaskList)
	}

	if isPaused(instance) {
		if instance.Status.Paused {
			return ctrl.Result{}, nil
		}
		reqLogger.Info(fmt.Sprintf("Pausing Tekton operator on resource: %v/%v", instance.Namespace, instance.Name))
		instance.Status.Paused = true
		instance.Status.ObservedGeneration = instance.Generation
		err := r.client.Status().Update(ctx, instance)
		return ctrl.Result{}, err
	}

	tektonRequest.Logger.V(1).Info("Updating CR status prior to operand reconciliation...")
	err = preUpdateStatus(tektonRequest)
	if err != nil {
		return handleError(tektonRequest, err)
	}
	tektonRequest.Logger.V(1).Info("CR status updated")

	reconcileResults := []common.ReconcileResult{}
	if tektonRequest.Instance.Spec.FeatureGates.DeployTektonTaskResources {
		tektonRequest.Logger.Info("Reconciling operands...")
		reconcileResults, err = r.reconcileOperands(tektonRequest)
		if err != nil {
			return handleError(tektonRequest, err)
		}
		tektonRequest.Logger.V(1).Info("Operands reconciled")
	} else {
		tektonRequest.Logger.V(1).Info("Resources were not deployed, because spec.featureGates.deployTektonTaskResources is set to false")
	}

	tektonRequest.Logger.V(1).Info("Updating CR status post reconciliation...")
	err = updateStatus(tektonRequest, reconcileResults)
	if err != nil {
		return handleError(tektonRequest, err)
	}
	tektonRequest.Logger.Info("CR status updated")

	if tektonRequest.Instance.Status.Phase == lifecycleapi.PhaseDeployed {
		common.TektonOperatorReconcilingProperly.Set(1)
	} else {
		common.TektonOperatorReconcilingProperly.Set(0)
	}

	return ctrl.Result{}, nil
}

func (r *tektonTasksReconciler) getCRList(ctx context.Context) (*tekton.TektonTasksList, error) {
	tektonTasksCRList := &tekton.TektonTasksList{}

	err := r.client.List(ctx, tektonTasksCRList)
	if err != nil {
		return nil, err
	}

	return tektonTasksCRList, nil
}

func (r *tektonTasksReconciler) setStatusMultipleCRs(ctx context.Context, tektonTasksList *tekton.TektonTasksList) error {
	const errMsg = "there are multiple CRs deployed"
	r.log.Error(nil, errMsg)

	for _, tektonTask := range tektonTasksList.Items {
		conditionsv1.SetStatusCondition(&tektonTask.Status.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: errMsg,
		})
		conditionsv1.SetStatusCondition(&tektonTask.Status.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionFalse,
			Reason:  "Progressing",
			Message: errMsg,
		})
		conditionsv1.SetStatusCondition(&tektonTask.Status.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: errMsg,
		})

		err := r.client.Status().Update(ctx, &tektonTask)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *tektonTasksReconciler) reconcileOperands(tektonRequest *common.Request) ([]common.ReconcileResult, error) {
	// Reconcile all operands
	allReconcileResults := make([]common.ReconcileResult, 0, len(r.operands))
	for _, operand := range r.operands {
		tektonRequest.Logger.V(1).Info(fmt.Sprintf("Reconciling operand: %s", operand.Name()))
		reconcileResults, err := operand.Reconcile(tektonRequest)
		if err != nil {
			tektonRequest.Logger.Info(fmt.Sprintf("Operand reconciliation failed: %s", err.Error()))
			return nil, err
		}
		allReconcileResults = append(allReconcileResults, reconcileResults...)
	}

	return allReconcileResults, nil
}

func (r *tektonTasksReconciler) setupController(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr)

	watchTektonResource(builder)

	watchClusterResources(builder, r.operands)

	watchNamespacedResources(builder, r.operands)

	return builder.Complete(r)
}

// SetupWithManager sets up the controller with the Manager.
func (r *tektonTasksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tekton.TektonTasks{}).
		Complete(r)
}

func (r *tektonTasksReconciler) clearCacheIfNeeded(tektonObj *tekton.TektonTasks) {
	if !reflect.DeepEqual(r.lastTektonTasksSpec, tektonObj.Spec) {
		r.subresourceCache = common.VersionCache{}
		r.lastTektonTasksSpec = tektonObj.Spec
	}
}

func (r *tektonTasksReconciler) clearCache() {
	r.lastTektonTasksSpec = tekton.TektonTasksSpec{}
	r.subresourceCache = common.VersionCache{}
}

func isPaused(object metav1.Object) bool {
	if object.GetAnnotations() == nil {
		return false
	}
	pausedStr, ok := object.GetAnnotations()[tekton.OperatorPausedAnnotation]
	if !ok {
		return false
	}
	paused, err := strconv.ParseBool(pausedStr)
	if err != nil {
		return false
	}
	return paused
}

func isBeingDeleted(object metav1.Object) bool {
	return !object.GetDeletionTimestamp().IsZero()
}

func isInitialized(tekton *tekton.TektonTasks) bool {
	return isBeingDeleted(tekton) || tekton.Status.Phase != lifecycleapi.PhaseEmpty
}

func initialize(request *common.Request) error {
	controllerutil.AddFinalizer(request.Instance, finalizerName)
	return updateTektonTasksResource(request)
}

func updateTektonTasks(request *common.Request) (bool, error) {
	updated := false

	// Update old finalizer to new one
	if controllerutil.ContainsFinalizer(request.Instance, oldFinalizerName) {
		controllerutil.RemoveFinalizer(request.Instance, oldFinalizerName)
		controllerutil.AddFinalizer(request.Instance, finalizerName)
		updated = true
	}

	if !updated {
		return false, nil
	}

	err := updateTektonTasksResource(request)
	return err == nil, err
}

func updateTektonTasksResource(request *common.Request) error {
	err := request.Client.Update(request.Context, request.Instance)
	if err != nil {
		return err
	}

	request.Instance.Status.Phase = lifecycleapi.PhaseDeploying
	request.Instance.Status.ObservedGeneration = request.Instance.Generation
	return request.Client.Status().Update(request.Context, request.Instance)
}

func (r *tektonTasksReconciler) cleanup(request *common.Request) error {
	if controllerutil.ContainsFinalizer(request.Instance, finalizerName) ||
		controllerutil.ContainsFinalizer(request.Instance, oldFinalizerName) {
		tektonStatus := &request.Instance.Status
		tektonStatus.Phase = lifecycleapi.PhaseDeleting
		tektonStatus.ObservedGeneration = request.Instance.Generation
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: "Deleting Tekton tasks resources",
		})
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: "Deleting Tekton tasks resources",
		})
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: "Deleting Tekton tasks resources",
		})

		err := request.Client.Status().Update(request.Context, request.Instance)
		if err != nil {
			return err
		}

		pendingCount := 0
		for _, operand := range r.operands {
			cleanupResults, err := operand.Cleanup(request)
			if err != nil {
				return err
			}

			for _, result := range cleanupResults {
				if !result.Deleted {
					pendingCount += 1
				}
			}
		}

		if pendingCount > 0 {
			// Will retry cleanup on next reconciliation iteration
			return nil
		}

		controllerutil.RemoveFinalizer(request.Instance, finalizerName)
		controllerutil.RemoveFinalizer(request.Instance, oldFinalizerName)
		err = request.Client.Update(request.Context, request.Instance)
		if err != nil {
			return err
		}
	}

	request.Instance.Status.Phase = lifecycleapi.PhaseDeleted
	request.Instance.Status.ObservedGeneration = request.Instance.Generation
	err := request.Client.Status().Update(request.Context, request.Instance)
	if errors.IsConflict(err) || errors.IsNotFound(err) {
		// These errors are ignored. They can happen if the CR was removed
		// before the status update call is executed.
		return nil
	}
	return err
}

func pauseCRs(tektonRequest *common.Request, kinds []string) error {
	patch := []byte(`{
  "metadata":{
    "annotations":{"kubevirt.io/operator.paused": "true"}
  }
}`)
	for _, kind := range kinds {
		crs := &unstructured.UnstructuredList{}
		crs.SetKind(kind)
		crs.SetAPIVersion("tektontasks.kubevirt.io/v1")
		err := tektonRequest.Client.List(tektonRequest.Context, crs)
		if err != nil {
			tektonRequest.Logger.Error(err, fmt.Sprintf("Error listing %s CRs: %s", kind, err))
			return err
		}
		for _, item := range crs.Items {
			err = tektonRequest.Client.Patch(tektonRequest.Context, &item, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				// Patching failed, maybe the CR just got removed? Log an error but keep going.
				tektonRequest.Logger.Error(err, fmt.Sprintf("Error pausing %s from namespace %s: %s",
					item.GetName(), item.GetNamespace(), err))
			}
		}
	}

	return nil
}

func preUpdateStatus(request *common.Request) error {
	operatorVersion := environment.GetOperatorVersion()

	tektonStatus := &request.Instance.Status
	tektonStatus.Phase = lifecycleapi.PhaseDeploying
	tektonStatus.ObservedGeneration = request.Instance.Generation
	tektonStatus.OperatorVersion = operatorVersion
	tektonStatus.TargetVersion = operatorVersion

	if tektonStatus.Paused {
		request.Logger.Info(fmt.Sprintf("Unpausing Tekton tasks operator on resource: %v/%v",
			request.Instance.Namespace, request.Instance.Name))
	}
	tektonStatus.Paused = false

	if !conditionsv1.IsStatusConditionPresentAndEqual(tektonStatus.Conditions, conditionsv1.ConditionAvailable, v1.ConditionFalse) {
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: "Reconciling Tekton tasks resources",
		})
	}

	if !conditionsv1.IsStatusConditionPresentAndEqual(tektonStatus.Conditions, conditionsv1.ConditionProgressing, v1.ConditionTrue) {
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: "Reconciling Tekton tasks resources",
		})
	}

	if !conditionsv1.IsStatusConditionPresentAndEqual(tektonStatus.Conditions, conditionsv1.ConditionDegraded, v1.ConditionTrue) {
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: "Reconciling Tekton tasks resources",
		})
	}

	return request.Client.Status().Update(request.Context, request.Instance)
}

func updateStatus(request *common.Request, reconcileResults []common.ReconcileResult) error {
	notAvailable := make([]common.ReconcileResult, 0, len(reconcileResults))
	progressing := make([]common.ReconcileResult, 0, len(reconcileResults))
	degraded := make([]common.ReconcileResult, 0, len(reconcileResults))
	for _, reconcileResult := range reconcileResults {
		if reconcileResult.Status.NotAvailable != nil {
			notAvailable = append(notAvailable, reconcileResult)
		}
		if reconcileResult.Status.Progressing != nil {
			progressing = append(progressing, reconcileResult)
		}
		if reconcileResult.Status.Degraded != nil {
			degraded = append(degraded, reconcileResult)
		}
	}

	tektonStatus := &request.Instance.Status
	switch len(notAvailable) {
	case 0:
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionTrue,
			Reason:  "Available",
			Message: "Tekton operator is available",
		})
	case 1:
		reconcileResult := notAvailable[0]
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: prefixResourceTypeAndName(*reconcileResult.Status.NotAvailable, reconcileResult.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionAvailable,
			Status:  v1.ConditionFalse,
			Reason:  "Available",
			Message: fmt.Sprintf("%d Tekton tasks resources are not available", len(notAvailable)),
		})
	}

	switch len(progressing) {
	case 0:
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionFalse,
			Reason:  "Progressing",
			Message: "No Tekton tasks resources are progressing",
		})
	case 1:
		reconcileResult := progressing[0]
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: prefixResourceTypeAndName(*reconcileResult.Status.Progressing, reconcileResult.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionProgressing,
			Status:  v1.ConditionTrue,
			Reason:  "Progressing",
			Message: fmt.Sprintf("%d Tekton tasks resources are progressing", len(progressing)),
		})
	}

	switch len(degraded) {
	case 0:
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionFalse,
			Reason:  "Degraded",
			Message: "No Tekton tasks resources are degraded",
		})
	case 1:
		reconcileResult := degraded[0]
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: prefixResourceTypeAndName(*reconcileResult.Status.Degraded, reconcileResult.Resource),
		})
	default:
		conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
			Type:    conditionsv1.ConditionDegraded,
			Status:  v1.ConditionTrue,
			Reason:  "Degraded",
			Message: fmt.Sprintf("%d Tekton tasks resources are degraded", len(degraded)),
		})
	}

	tektonStatus.ObservedGeneration = request.Instance.Generation
	if len(notAvailable) == 0 && len(progressing) == 0 && len(degraded) == 0 {
		tektonStatus.Phase = lifecycleapi.PhaseDeployed
		tektonStatus.ObservedVersion = environment.GetOperatorVersion()
	} else {
		tektonStatus.Phase = lifecycleapi.PhaseDeploying
	}

	return request.Client.Status().Update(request.Context, request.Instance)
}

func prefixResourceTypeAndName(message string, resource client.Object) string {
	return fmt.Sprintf("%s %s/%s: %s",
		resource.GetObjectKind().GroupVersionKind().Kind,
		resource.GetNamespace(),
		resource.GetName(),
		message)
}

func handleError(request *common.Request, errParam error) (ctrl.Result, error) {
	if errParam == nil {
		return ctrl.Result{}, nil
	}

	if errors.IsConflict(errParam) {
		// Conflict happens if multiple components modify the same resource.
		// Ignore the error and restart reconciliation.
		return ctrl.Result{Requeue: true}, nil
	}

	// Default error handling, if error is not known
	errorMsg := fmt.Sprintf("Error: %v", errParam)
	tektonStatus := &request.Instance.Status
	tektonStatus.Phase = lifecycleapi.PhaseDeploying
	conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionAvailable,
		Status:  v1.ConditionFalse,
		Reason:  "Available",
		Message: errorMsg,
	})
	conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionProgressing,
		Status:  v1.ConditionTrue,
		Reason:  "Progressing",
		Message: errorMsg,
	})
	conditionsv1.SetStatusCondition(&tektonStatus.Conditions, conditionsv1.Condition{
		Type:    conditionsv1.ConditionDegraded,
		Status:  v1.ConditionTrue,
		Reason:  "Degraded",
		Message: errorMsg,
	})
	err := request.Client.Status().Update(request.Context, request.Instance)
	if err != nil {
		request.Logger.Error(err, "Error updating Tekton tasks status.")
	}

	return ctrl.Result{}, errParam
}

func watchTektonResource(bldr *ctrl.Builder) {
	// Predicate is used to only reconcile on these changes to the Tekton tasks resource:
	// - any change in spec - checked with generation
	// - deletion timestamp - to trigger cleanup when Tekton tasks CR is being deleted
	// - labels or annotations - to detect if reconciliation should be paused or unpaused
	// - finalizers - to trigger reconciliation after initialization
	//
	// Importantly, the reconciliation is not triggered on status change.
	// Otherwise it would cause a reconciliation loop.
	pred := predicate.Funcs{UpdateFunc: func(event event.UpdateEvent) bool {
		oldObj := event.ObjectOld
		newObj := event.ObjectNew
		return newObj.GetGeneration() != oldObj.GetGeneration() ||
			!newObj.GetDeletionTimestamp().Equal(oldObj.GetDeletionTimestamp()) ||
			!reflect.DeepEqual(newObj.GetLabels(), oldObj.GetLabels()) ||
			!reflect.DeepEqual(newObj.GetAnnotations(), oldObj.GetAnnotations()) ||
			!reflect.DeepEqual(newObj.GetFinalizers(), oldObj.GetFinalizers())

	}}

	bldr.For(&tekton.TektonTasks{}, builder.WithPredicates(pred))
}

func watchNamespacedResources(builder *ctrl.Builder, tektonOperands []operands.Operand) {
	watchResources(builder,
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &tekton.TektonTasks{},
		},
		tektonOperands,
		operands.Operand.WatchTypes,
	)
}

func watchClusterResources(builder *ctrl.Builder, tektonOperands []operands.Operand) {
	watchResources(builder,
		&libhandler.EnqueueRequestForAnnotation{
			Type: schema.GroupKind{
				Group: tekton.GroupVersion.Group,
				Kind:  "TektonTasks",
			},
		},
		tektonOperands,
		operands.Operand.WatchClusterTypes,
	)
}

func watchResources(builder *ctrl.Builder, handler handler.EventHandler, tektonOperands []operands.Operand, watchTypesFunc func(operands.Operand) []client.Object) {
	watchedTypes := make(map[reflect.Type]struct{})
	for _, operand := range tektonOperands {
		for _, t := range watchTypesFunc(operand) {
			if _, ok := watchedTypes[reflect.TypeOf(t)]; ok {
				continue
			}

			builder.Watches(&source.Kind{Type: t}, handler)
			watchedTypes[reflect.TypeOf(t)] = struct{}{}
		}
	}
}
