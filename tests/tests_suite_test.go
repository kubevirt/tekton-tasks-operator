package tests

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	. "github.com/onsi/ginkgo/v2"
	ginkgo_reporters "github.com/onsi/ginkgo/v2/reporters"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1api "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	v1reporter "kubevirt.io/client-go/reporter"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	qe_reporters "kubevirt.io/qe-tools/pkg/ginkgo-reporters"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	tenSecondTimeout       = 10 * time.Second
	timeout                = 10 * time.Minute
	testScheme             *runtime.Scheme
	envExistingCrNamespace = "TEST_EXISTING_CR_NAMESPACE"
)

type TestSuiteStrategy interface {
	Init()

	GetName() string
	GetNamespace() string
	GetTTO() *tekton.TektonTasks

	GetVersionLabel() string
	GetPartOfLabel() string
}

type newTektonStrategy struct {
	tekton *tekton.TektonTasks
}

var _ TestSuiteStrategy = &newTektonStrategy{}

func (t *newTektonStrategy) Init() {
	t.CreateTTOIfNeeded()
}

func (t *newTektonStrategy) CreateTTOIfNeeded() {
	ttos := tekton.TektonTasksList{}
	err := apiClient.List(ctx, &ttos)
	Expect(err).ToNot(HaveOccurred())
	if len(ttos.Items) == 0 {
		newTekton := &tekton.TektonTasks{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.GetName(),
				Namespace: t.GetNamespace(),
				Labels: map[string]string{
					common.AppKubernetesNameLabel:      "tto-cr",
					common.AppKubernetesManagedByLabel: "tto-test-strategy",
					common.AppKubernetesPartOfLabel:    "hyperconverged-cluster",
					common.AppKubernetesVersionLabel:   "v0.0.0-test",
					common.AppKubernetesComponentLabel: common.AppComponentTektonTasks.String(),
				},
			},
			Spec: tekton.TektonTasksSpec{
				Pipelines: tekton.Pipelines{
					Namespace: t.GetNamespace(),
				},
				FeatureGates: tekton.FeatureGates{
					DeployTektonTaskResources: false,
				},
			},
		}

		Eventually(func() error {
			return apiClient.Create(ctx, newTekton)
		}, timeout, time.Second).ShouldNot(HaveOccurred())
		t.tekton = newTekton
	} else {
		t.tekton = &(ttos.Items[0])
	}
}

func (t *newTektonStrategy) GetName() string {
	if t.tekton == nil {
		return "tto-test"
	}
	return t.tekton.Name
}

func (t *newTektonStrategy) GetTTO() *tekton.TektonTasks {
	return t.tekton
}

func (t *newTektonStrategy) GetNamespace() string {
	if t.tekton == nil {
		if existingCrNamespace := os.Getenv(envExistingCrNamespace); existingCrNamespace != "" {
			return existingCrNamespace
		}
		//return default namespace
		return "kubevirt"
	}
	return t.tekton.Namespace
}

func (t *newTektonStrategy) GetVersionLabel() string {
	return t.tekton.Labels[common.AppKubernetesVersionLabel]
}
func (t *newTektonStrategy) GetPartOfLabel() string {
	return t.tekton.Labels[common.AppKubernetesPartOfLabel]
}

var (
	apiClient          client.Client
	ctx                context.Context
	strategy           = newTektonStrategy{}
	deploymentTimedOut bool
)

var _ = BeforeSuite(func() {
	testScheme = runtime.NewScheme()
	setupApiClient()
	strategy.Init()

	// Wait to finish deployment before running any tests
	waitUntilDeployed()
})

var _ = AfterSuite(func() {
	tektonTasksCRList := &tekton.TektonTasksList{}
	apiClient.List(ctx, tektonTasksCRList)
	for _, tto := range tektonTasksCRList.Items {
		deleteTekton(&tto)
	}
})

func setupApiClient() {
	Expect(tekton.AddToScheme(testScheme)).ToNot(HaveOccurred())
	Expect(pipeline.AddToScheme(testScheme)).ToNot(HaveOccurred())
	Expect(clientgoscheme.AddToScheme(testScheme)).ToNot(HaveOccurred())

	cfg, err := config.GetConfig()
	Expect(err).ToNot(HaveOccurred())
	apiClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).ToNot(HaveOccurred())

	ctx = context.Background()
}

func getTekton() *tekton.TektonTasks {
	key := client.ObjectKey{Name: strategy.GetName(), Namespace: strategy.GetNamespace()}
	foundTekton := &tekton.TektonTasks{}
	Expect(apiClient.Get(ctx, key, foundTekton)).ToNot(HaveOccurred())
	return foundTekton
}

func waitUntilDeployed() {
	if deploymentTimedOut {
		Fail("Timed out waiting for TTO to be in phase Deployed.")
	}

	// Set to true before waiting. In case Eventually fails,
	// it will panic and the deploymentTimedOut will be left true
	deploymentTimedOut = true
	EventuallyWithOffset(1, func() bool {
		tekton := getTekton()
		return tekton.Status.Phase == lifecycleapi.PhaseDeployed
	}, timeout, time.Second).Should(BeTrue())
	deploymentTimedOut = false
}

func deleteTekton(tto *tekton.TektonTasks) {
	apiClient.Delete(ctx, tto)
	Eventually(func() v1api.StatusReason {
		namespacedName := types.NamespacedName{
			Namespace: tto.Namespace,
			Name:      tto.Name,
		}
		foundTekton := &tekton.TektonTasks{}
		err := apiClient.Get(ctx, namespacedName, foundTekton)
		return errors.ReasonForError(err)
	}, 60*time.Second, 5*time.Second).Should(Equal(v1api.StatusReasonNotFound), "it should not find CR")
}

func (t *newTektonStrategy) createTekton(name string) *tekton.TektonTasks {
	tekton := strategy.GetTTO()
	tekton.Name = name
	tekton.Spec.FeatureGates.DeployTektonTaskResources = true
	createOrUpdateTekton(tekton)

	return tekton
}

func createOrUpdateTekton(tek *tekton.TektonTasks) {
	key := client.ObjectKey{
		Name:      tek.Name,
		Namespace: tek.Namespace,
	}
	Eventually(func() error {
		foundTekton := &tekton.TektonTasks{}
		err := apiClient.Get(ctx, key, foundTekton)
		if err == nil {
			isEqual := reflect.DeepEqual(foundTekton.Spec, tek.Spec) &&
				reflect.DeepEqual(foundTekton.ObjectMeta.Annotations, tek.ObjectMeta.Annotations) &&
				reflect.DeepEqual(foundTekton.ObjectMeta.Labels, tek.ObjectMeta.Labels)
			if isEqual {
				return nil
			}
			foundTekton.Spec = tek.Spec
			foundTekton.Annotations = tek.Annotations
			foundTekton.Labels = tek.Labels
			strategy.tekton = foundTekton

			return apiClient.Update(ctx, foundTekton)
		}
		if errors.IsNotFound(err) {
			newTTO := &tekton.TektonTasks{
				ObjectMeta: metav1.ObjectMeta{
					Name:        tek.Name,
					Namespace:   tek.Namespace,
					Annotations: tek.Annotations,
					Labels:      tek.Labels,
				},
				Spec: tek.Spec,
			}
			strategy.tekton = newTTO
			return apiClient.Create(ctx, newTTO)
		}
		return err
	}, timeout, time.Second).ShouldNot(HaveOccurred())
}

var afterSuiteReporters []Reporter

func TestFunctional(t *testing.T) {
	if qe_reporters.JunitOutput != "" {
		afterSuiteReporters = append(afterSuiteReporters, v1reporter.NewV1JUnitReporter(qe_reporters.JunitOutput))
	}

	if qe_reporters.Polarion.Run {
		afterSuiteReporters = append(afterSuiteReporters, &qe_reporters.Polarion)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional test suite")
}

var _ = ReportAfterSuite("TestFunctional", func(report Report) {
	for _, reporter := range afterSuiteReporters {
		ginkgo_reporters.ReportViaDeprecatedReporter(reporter, report)
	}
})
