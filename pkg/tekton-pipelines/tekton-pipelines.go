package tekton_pipelines

import (
	"fmt"

	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	"github.com/kubevirt/tekton-tasks-operator/pkg/environment"
	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
	tektonbundle "github.com/kubevirt/tekton-tasks-operator/pkg/tekton-bundle"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=tekton.dev,resources=pipelines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=*,resources=configmaps,verbs=list;watch;create;delete

const (
	operandName      = "tekton-pipelines"
	operandComponent = common.AppComponentTektonPipelines
)

var requiredCRDs = []string{"tasks.tekton.dev"}

func init() {
	utilruntime.Must(pipeline.AddToScheme(common.Scheme))
}

type tektonPipelines struct {
	pipelines  []pipeline.Pipeline
	configMaps []v1.ConfigMap
}

var _ operands.Operand = &tektonPipelines{}

func New(bundle *tektonbundle.Bundle) *tektonPipelines {
	tp := &tektonPipelines{
		pipelines:  bundle.Pipelines,
		configMaps: bundle.ConfigMaps,
	}
	return tp
}

func (t *tektonPipelines) Name() string {
	return operandName
}

func (t *tektonPipelines) WatchClusterTypes() []client.Object {
	return nil
}

func (t *tektonPipelines) WatchTypes() []client.Object {
	return []client.Object{
		&pipeline.Pipeline{},
		&v1.ConfigMap{},
	}
}

func (t *tektonPipelines) RequiredCrds() []string {
	return requiredCRDs
}

func (t *tektonPipelines) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	var results []common.ReconcileResult
	var reconcileFunc []common.ReconcileFunc
	reconcileFunc = append(reconcileFunc, reconcileTektonPipelinesFuncs(t.pipelines)...)
	reconcileFunc = append(reconcileFunc, reconcileConfigMapsFuncs(t.configMaps)...)

	reconcileTektonBundleResults, err := common.CollectResourceStatus(request, reconcileFunc...)
	if err != nil {
		return nil, err
	}

	upgradingNow := isUpgradingNow(request)
	for _, r := range reconcileTektonBundleResults {
		if !upgradingNow && (r.OperationResult == common.OperationResultUpdated) {
			request.Logger.Info(fmt.Sprintf("Changes reverted in tekton tasks: %s", r.Resource.GetName()))
		}
	}
	return append(results, reconcileTektonBundleResults...), nil
}

func (t *tektonPipelines) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object
	for _, p := range t.pipelines {
		objects = append(objects, &p)
	}
	for _, cm := range t.configMaps {
		objects = append(objects, &cm)
	}

	return common.DeleteAll(request, objects...)
}

func isUpgradingNow(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != environment.GetOperatorVersion()
}

func reconcileTektonPipelinesFuncs(pipelines []pipeline.Pipeline) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(pipelines))
	for i := range pipelines {
		p := &pipelines[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Spec.Pipelines.Namespace
			p.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(p).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newPipeline := newRes.(*pipeline.Pipeline)
					foundPipeline := foundRes.(*pipeline.Pipeline)
					foundPipeline.Spec = newPipeline.Spec
				}).
				Reconcile()
		})
	}
	return funcs
}

func reconcileConfigMapsFuncs(configMaps []v1.ConfigMap) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(configMaps))
	for i := range configMaps {
		cm := &configMaps[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Spec.Pipelines.Namespace
			cm.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(cm).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newCM := newRes.(*v1.ConfigMap)
					foundCM := foundRes.(*v1.ConfigMap)
					foundCM.Data = newCM.Data
				}).
				Reconcile()
		})
	}
	return funcs
}
