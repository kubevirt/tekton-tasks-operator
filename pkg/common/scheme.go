package common

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// Scheme used for the tekton operator.
	Scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(extv1.AddToScheme(Scheme))
	utilruntime.Must(tekton.AddToScheme(Scheme))
	utilruntime.Must(openshiftconfigv1.AddToScheme(Scheme))
}
