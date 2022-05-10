package controllers

import (
	"context"

	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	tektonbundle "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-bundle"
	tektonpipelines "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-pipelines"
	tektontasks "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-tasks"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func CreateAndSetupReconciler(mgr controllerruntime.Manager) error {
	reader := mgr.GetAPIReader()
	ctx := context.Background()
	ttTasksBundle, err := tektonbundle.ReadTasksBundle(reader, ctx)
	if err != nil {
		return err
	}
	ttPipelinesBundle, err := tektonbundle.ReadPipelineBundle(reader, ctx)
	if err != nil {
		return err
	}

	tektonOperands := []operands.Operand{
		tektontasks.New(ttTasksBundle),
		tektonpipelines.New(ttPipelinesBundle),
	}

	var requiredCrds []string
	for i := range tektonOperands {
		requiredCrds = append(requiredCrds, tektonOperands[i].RequiredCrds()...)
	}

	// Check if all needed CRDs exist
	crdList := &extv1.CustomResourceDefinitionList{}
	err = mgr.GetAPIReader().List(context.TODO(), crdList)
	if err != nil {
		return err
	}

	reconciler := NewTektonReconciler(mgr.GetClient(), mgr.GetAPIReader(), tektonOperands)

	if requiredCrdsExist(requiredCrds, crdList.Items) {
		// No need to start CRD controller
		return reconciler.setupController(mgr)
	}

	mgr.GetLogger().Info("Required CRDs do not exist. Waiting until they are installed.",
		"required_crds", requiredCrds,
	)

	crdController, err := CreateCrdController(mgr, requiredCrds)
	if err != nil {
		return err
	}

	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		// First start the CRD controller
		err := crdController.Start(ctx)
		if err != nil {
			return err
		}

		mgr.GetLogger().Info("Required CRDs were installed, starting tekton-tasks operator.")

		// Clear variable, so it can be garbage collected
		crdController = nil

		// After it is finished, add the tekton controller to the manager
		return reconciler.setupController(mgr)
	}))
}

func requiredCrdsExist(required []string, foundCrds []extv1.CustomResourceDefinition) bool {
OuterLoop:
	for i := range required {
		for j := range foundCrds {
			if required[i] == foundCrds[j].Name {
				continue OuterLoop
			}
		}
		return false
	}
	return true
}
