package tests

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	tekton "github.com/kubevirt/tekton-tasks-operator/api/v1alpha1"
	"github.com/kubevirt/tekton-tasks-operator/pkg/common"
	. "github.com/onsi/ginkgo/v2"
	ginkgo_reporters "github.com/onsi/ginkgo/v2/reporters"
	. "github.com/onsi/gomega"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	qe_reporters "kubevirt.io/qe-tools/pkg/ginkgo-reporters"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	envExistingCrName        = "TEST_EXISTING_CR_NAME"
	envExistingCrNamespace   = "TEST_EXISTING_CR_NAMESPACE"
	envSkipUpdateTektonTests = "SKIP_UPDATE_TEKTON_TESTS"
	envSkipCleanupAfterTests = "SKIP_CLEANUP_AFTER_TESTS"
	envTimeout               = "TIMEOUT_MINUTES"
	envShortTimeout          = "SHORT_TIMEOUT_MINUTES"
	envTopologyMode          = "TOPOLOGY_MODE"
)

var (
	tenSecondTimeout = 10 * time.Second
	shortTimeout     = 1 * time.Minute
	timeout          = 10 * time.Minute
	testScheme       *runtime.Scheme
)

type TestSuiteStrategy interface {
	Init()
	Cleanup()

	GetName() string
	GetNamespace() string
	GetPipelinesNamespace() string

	GetVersionLabel() string
	GetPartOfLabel() string

	RevertToOriginalTektonCr()
	SkipTektonUpdateTestsIfNeeded()
}

type newTektonStrategy struct {
	tekton *tekton.TektonTasks
}

var _ TestSuiteStrategy = &newTektonStrategy{}

func (t *newTektonStrategy) Init() {
	Eventually(func() error {
		namespaceObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: t.GetNamespace()}}
		return apiClient.Create(ctx, namespaceObj)
	}, timeout, time.Second).ShouldNot(HaveOccurred())

	Eventually(func() error {
		namespaceObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: t.GetPipelinesNamespace()}}
		return apiClient.Create(ctx, namespaceObj)
	}, timeout, time.Second).ShouldNot(HaveOccurred())

	newTekton := &tekton.TektonTasks{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.GetName(),
			Namespace: t.GetNamespace(),
			Labels: map[string]string{
				common.AppKubernetesNameLabel:      "ssp-cr",
				common.AppKubernetesManagedByLabel: "ssp-test-strategy",
				common.AppKubernetesPartOfLabel:    "hyperconverged-cluster",
				common.AppKubernetesVersionLabel:   "v0.0.0-test",
				common.AppKubernetesComponentLabel: common.AppComponentTektonTasks.String(),
			},
		},
		Spec: tekton.TektonTasksSpec{
			Pipelines: tekton.Pipelines{
				Namespace: t.GetPipelinesNamespace(),
			},
		},
	}

	Eventually(func() error {
		return apiClient.Create(ctx, newTekton)
	}, timeout, time.Second).ShouldNot(HaveOccurred())
	t.tekton = newTekton
}

func (t *newTektonStrategy) Cleanup() {
	if getBoolEnv(envSkipCleanupAfterTests) {
		return
	}

	if t.tekton != nil {
		err := apiClient.Delete(ctx, t.tekton)
		expectSuccessOrNotFound(err)
		waitForDeletion(client.ObjectKey{
			Name:      t.GetName(),
			Namespace: t.GetNamespace(),
		}, &tekton.TektonTasks{})
	}

	err1 := apiClient.Delete(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: t.GetNamespace()}})
	err2 := apiClient.Delete(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: t.GetPipelinesNamespace()}})
	expectSuccessOrNotFound(err1)
	expectSuccessOrNotFound(err2)

	waitForDeletion(client.ObjectKey{Name: t.GetNamespace()}, &v1.Namespace{})
	waitForDeletion(client.ObjectKey{Name: t.GetPipelinesNamespace()}, &v1.Namespace{})
}

func (t *newTektonStrategy) GetName() string {
	return "test-tekton"
}

func (t *newTektonStrategy) GetNamespace() string {
	const testNamespace = "tekton-tasks-functests"
	return testNamespace
}

func (t *newTektonStrategy) GetPipelinesNamespace() string {
	const pipelinesTestNt = "tekton-tasks-operator-functests-pipelines"
	return pipelinesTestNt
}

func (t *newTektonStrategy) GetVersionLabel() string {
	return t.tekton.Labels[common.AppKubernetesVersionLabel]
}
func (t *newTektonStrategy) GetPartOfLabel() string {
	return t.tekton.Labels[common.AppKubernetesPartOfLabel]
}

func (t *newTektonStrategy) RevertToOriginalTektonCr() {
	waitForTektonTasksOperatorDeletionIfNeeded(t.tekton)
	createOrUpdateTekton(t.tekton)
}

func (t *newTektonStrategy) SkipTektonUpdateTestsIfNeeded() {
	// Do not skip tekton update testt in thit strategy
	return
}

type existingTektontrategy struct {
	Name      string
	Namespace string

	tekton *tekton.TektonTasks
}

var _ TestSuiteStrategy = &existingTektontrategy{}

func (t *existingTektontrategy) Init() {
	existingTekton := &tekton.TektonTasks{}
	err := apiClient.Get(ctx, client.ObjectKey{Name: t.Name, Namespace: t.Namespace}, existingTekton)
	Expect(err).ToNot(HaveOccurred())

	pipelinesNamespace := existingTekton.Spec.Pipelines.Namespace
	Expect(apiClient.Get(ctx, client.ObjectKey{Name: pipelinesNamespace}, &v1.Namespace{}))

	t.tekton = existingTekton

	if t.tektonModificationDisabled() {
		return
	}
	// Try to modify the tekton-tasks cr and check if it it not reverted by another operator
	defer t.RevertToOriginalTektonCr()

}

func (t *existingTektontrategy) Cleanup() {
	if t.tekton != nil {
		t.RevertToOriginalTektonCr()
	}
}

func (t *existingTektontrategy) GetName() string {
	return t.Name
}

func (t *existingTektontrategy) GetNamespace() string {
	return t.Namespace
}

func (t *existingTektontrategy) GetPipelinesNamespace() string {
	if t.tekton == nil {
		panic("Strategy it not initialized")
	}
	return t.tekton.Spec.Pipelines.Namespace
}

func (t *existingTektontrategy) GetVersionLabel() string {
	return t.tekton.Labels[common.AppKubernetesVersionLabel]
}
func (t *existingTektontrategy) GetPartOfLabel() string {
	return t.tekton.Labels[common.AppKubernetesPartOfLabel]
}

func (t *existingTektontrategy) RevertToOriginalTektonCr() {
	waitForTektonTasksOperatorDeletionIfNeeded(t.tekton)
	createOrUpdateTekton(t.tekton)
}

func (t *existingTektontrategy) SkipTektonUpdateTestsIfNeeded() {
	if t.tektonModificationDisabled() {
		Skip("Testt that update SSP CR are disabled", 1)
	}
}

func (t *existingTektontrategy) tektonModificationDisabled() bool {
	return getBoolEnv(envSkipUpdateTektonTests)
}

var (
	apiClient          client.Client
	coreClient         *kubernetes.Clientset
	ctx                context.Context
	strategy           TestSuiteStrategy
	sspListerWatcher   cache.ListerWatcher
	deploymentTimedOut bool
)

var _ = BeforeSuite(func() {
	existingCrName := os.Getenv(envExistingCrName)
	if existingCrName == "" {
		strategy = &newTektonStrategy{}
	} else {
		existingCrNamespace := os.Getenv(envExistingCrNamespace)
		Expect(existingCrNamespace).ToNot(BeEmpty(), "Existing CR Namespace need to be defined")
		strategy = &existingTektontrategy{Name: existingCrName, Namespace: existingCrNamespace}
	}

	envTimeout, set := getIntEnv(envTimeout)
	if set {
		timeout = time.Duration(envTimeout) * time.Minute
		fmt.Println(fmt.Sprintf("timeout set to %d minutes", envTimeout))
	}

	envShortTimeout, set := getIntEnv(envShortTimeout)
	if set {
		shortTimeout = time.Duration(envShortTimeout) * time.Minute
		fmt.Println(fmt.Sprintf("short timeout set to %d minutes", envShortTimeout))
	}

	testScheme = runtime.NewScheme()
	setupApiClient()
	strategy.Init()

	// Wait to finish deployment before running any tests
	waitUntilDeployed()
})

var _ = AfterSuite(func() {
	strategy.Cleanup()
})

func expectSuccessOrNotFound(err error) {
	if err != nil && !errors.IsNotFound(err) {
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
	}
}

func setupApiClient() {
	Expect(tekton.AddToScheme(testScheme)).ToNot(HaveOccurred())
	Expect(pipeline.AddToScheme(testScheme)).ToNot(HaveOccurred())
	Expect(clientgoscheme.AddToScheme(testScheme)).ToNot(HaveOccurred())

	cfg, err := config.GetConfig()
	Expect(err).ToNot(HaveOccurred())
	apiClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).ToNot(HaveOccurred())
	coreClient, err = kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	ctx = context.Background()
	sspListerWatcher = createSspListerWatcher(cfg)
}

func createSspListerWatcher(cfg *rest.Config) cache.ListerWatcher {
	sspGvk, err := apiutil.GVKForObject(&tekton.TektonTasks{}, testScheme)
	Expect(err).ToNot(HaveOccurred())

	restClient, err := apiutil.RESTClientForGVK(sspGvk, false, cfg, serializer.NewCodecFactory(testScheme))
	Expect(err).ToNot(HaveOccurred())

	return cache.NewListWatchFromClient(restClient, "tektontasks", strategy.GetNamespace(), fields.Everything())
}

func getTekton() *tekton.TektonTasks {
	key := client.ObjectKey{Name: strategy.GetName(), Namespace: strategy.GetNamespace()}
	foundTekton := &tekton.TektonTasks{}
	Expect(apiClient.Get(ctx, key, foundTekton)).ToNot(HaveOccurred())
	return foundTekton
}

func waitUntilDeployed() {
	if deploymentTimedOut {
		Fail("Timed out waiting for SSP to be in phase Deployed.")
	}

	// Set to true before waiting. In case Eventually fails,
	// it will panic and the deploymentTimedOut will be left true
	deploymentTimedOut = true
	EventuallyWithOffset(1, func() bool {
		tekton := getTekton()
		return tekton.Status.ObservedGeneration == tekton.Generation &&
			tekton.Status.Phase == lifecycleapi.PhaseDeployed
	}, timeout, time.Second).Should(BeTrue())
	deploymentTimedOut = false
}

func waitForDeletion(key client.ObjectKey, obj client.Object) {
	EventuallyWithOffset(1, func() bool {
		err := apiClient.Get(ctx, key, obj)
		return errors.IsNotFound(err)
	}, timeout, time.Second).Should(BeTrue())
}

func getBoolEnv(envName string) bool {
	envVal := os.Getenv(envName)
	if envVal == "" {
		return false
	}
	val, err := strconv.ParseBool(envVal)
	if err != nil {
		return false
	}
	return val
}

// getIntEnv returnt (0, false) if an env var it not set or (X, true) if it it set
func getIntEnv(envName string) (int, bool) {
	envVal := os.Getenv(envName)
	if envVal == "" {
		return 0, false
	} else {
		val, err := strconv.ParseInt(envVal, 10, 32)
		if err != nil {
			panic(err)
		}
		return int(val), true
	}
}

func waitForTektonTasksOperatorDeletionIfNeeded(ssp *tekton.TektonTasks) {
	key := client.ObjectKey{Name: ssp.Name, Namespace: ssp.Namespace}
	Eventually(func() error {
		foundTekton := &tekton.TektonTasks{}
		err := apiClient.Get(ctx, key, foundTekton)
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if foundTekton.DeletionTimestamp != nil {
			return fmt.Errorf("waiting for SSP CR deletion")
		}
		return nil
	}, timeout, time.Second).ShouldNot(HaveOccurred())
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
			return apiClient.Update(ctx, foundTekton)
		}
		if errors.IsNotFound(err) {
			newSsp := &tekton.TektonTasks{
				ObjectMeta: metav1.ObjectMeta{
					Name:        tek.Name,
					Namespace:   tek.Namespace,
					Annotations: tek.Annotations,
					Labels:      tek.Labels,
				},
				Spec: tek.Spec,
			}
			return apiClient.Create(ctx, newSsp)
		}
		return err
	}, timeout, time.Second).ShouldNot(HaveOccurred())
}

func triggerReconciliation() {
	updateTekton(func(foundTekton *tekton.TektonTasks) {
		if foundTekton.GetAnnotations() == nil {
			foundTekton.SetAnnotations(map[string]string{})
		}

		foundTekton.GetAnnotations()["forceReconciliation"] = ""
	})

	updateTekton(func(foundTekton *tekton.TektonTasks) {
		delete(foundTekton.GetAnnotations(), "forceReconciliation")
	})

	// Wait a second to give time for operator to notice the change
	time.Sleep(time.Second)

	waitUntilDeployed()
}
func updateTekton(updateFunc func(foundTekton *tekton.TektonTasks)) {
	Eventually(func() error {
		foundTekton := getTekton()
		updateFunc(foundTekton)
		return apiClient.Update(ctx, foundTekton)
	}, timeout, time.Second).ShouldNot(HaveOccurred())
}
func TestFunctional(t *testing.T) {
	var reporters []Reporter

	if qe_reporters.JunitOutput != "" {
		reporters = append(reporters, ginkgo_reporters.NewJUnitReporter(qe_reporters.JunitOutput))
	}

	if qe_reporters.Polarion.Run {
		reporters = append(reporters, &qe_reporters.Polarion)
	}

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Functional test suite", reporters)
}
