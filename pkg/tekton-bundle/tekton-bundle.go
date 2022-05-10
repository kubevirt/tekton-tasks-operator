package tekton_bundle

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/kubevirt/tekton-tasks-operator/pkg/operands"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	yamlv2 "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	tektonTasksKubernetesBundleDir     = "/data/tekton-tasks/kubernetes/"
	tektonTasksOKDBundleDir            = "/data/tekton-tasks/okd/"
	tektonPipelinesKubernetesBundleDir = "/data/tekton-pipelines/kubernetes/"
	tektonPipelinesOKDBundleDir        = "/data/tekton-pipelines/okd/"
)

var (
	clusterTasksString = string(pipeline.ClusterTaskKind)
	pipelineKindString = "Pipeline"
	serviceAccountKind = rbac.ServiceAccountKind
	roleBindingKind    = "RoleBinding"
	clusterRoleKind    = "ClusterRole"
	configMapKind      = "ConfigMap"
)

type Bundle struct {
	ClusterTasks    []pipeline.ClusterTask
	ServiceAccounts []v1.ServiceAccount
	RoleBindings    []rbac.RoleBinding
	ClusterRoles    []rbac.ClusterRole
	Pipelines       []pipeline.Pipeline
	ConfigMaps      []v1.ConfigMap
}

func ReadTasksBundle(cl client.Reader, ctx context.Context) (*Bundle, error) {
	isOpenshift, err := runningOnOpenshift(cl, ctx)
	if err != nil {
		return nil, err
	}

	path := getTasksBundlePath(isOpenshift)
	files, err := readFile(path)
	if err != nil {
		return nil, err
	}

	tektonObjs, err := decodeObjectsFromFiles(files)
	if err != nil {
		return nil, err
	}

	return tektonObjs, nil
}

func ReadPipelineBundle(cl client.Reader, ctx context.Context) (*Bundle, error) {
	isOpenshift, err := runningOnOpenshift(cl, ctx)
	if err != nil {
		return nil, err
	}

	path := getPipelineBundlePath(isOpenshift)
	files, err := readFolder(path)
	if err != nil {
		return nil, err
	}

	tektonObjs, err := decodeObjectsFromFiles(files)
	if err != nil {
		return nil, err
	}

	return tektonObjs, nil
}

func getPipelineBundlePath(isOpenshift bool) string {
	if isOpenshift {
		return tektonPipelinesOKDBundleDir
	}
	return tektonPipelinesKubernetesBundleDir
}

func getTasksBundlePath(isOpenshift bool) string {
	if isOpenshift {
		return filepath.Join(tektonTasksOKDBundleDir, "kubevirt-tekton-tasks-okd-"+operands.TektonTasksVersion+".yaml")
	}
	return filepath.Join(tektonTasksKubernetesBundleDir, "kubevirt-tekton-tasks-kubernetes-"+operands.TektonTasksVersion+".yaml")
}

func runningOnOpenshift(cl client.Reader, ctx context.Context) (bool, error) {
	clusterVersion := &openshiftconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(clusterVersion), clusterVersion); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			// Not on OpenShift
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

func readFile(fileName string) ([][]byte, error) {
	file, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	return [][]byte{file}, nil
}

func readFolder(folderPath string) ([][]byte, error) {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}
	filesBytes := make([][]byte, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		f, err := ioutil.ReadFile(filepath.Join(folderPath, file.Name()))
		if err != nil {
			return nil, err
		}
		filesBytes = append(filesBytes, f)
	}

	return filesBytes, nil
}

func decodeObjectsFromFiles(files [][]byte) (*Bundle, error) {
	bundle := &Bundle{}
	for _, file := range files {
		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(file), 1024)
		for {
			var obj map[string]interface{}
			err := decoder.Decode(&obj)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if kind, ok := obj["kind"].(string); ok {
				if kind == "" {
					continue
				}

				switch kind {
				case clusterTasksString:
					clusterTask := pipeline.ClusterTask{}
					err = getObject(obj, &clusterTask)
					bundle.ClusterTasks = append(bundle.ClusterTasks, clusterTask)
				case pipelineKindString:
					p := pipeline.Pipeline{}
					err = getObject(obj, &p)
					bundle.Pipelines = append(bundle.Pipelines, p)
				case serviceAccountKind:
					sa := v1.ServiceAccount{}
					err = getObject(obj, &sa)
					bundle.ServiceAccounts = append(bundle.ServiceAccounts, sa)
				case roleBindingKind:
					rb := rbac.RoleBinding{}
					err = getObject(obj, &rb)
					bundle.RoleBindings = append(bundle.RoleBindings, rb)
				case clusterRoleKind:
					cr := rbac.ClusterRole{}
					err = getObject(obj, &cr)
					bundle.ClusterRoles = append(bundle.ClusterRoles, cr)
				case configMapKind:
					cm := v1.ConfigMap{}
					err = getObject(obj, &cm)
					bundle.ConfigMaps = append(bundle.ConfigMaps, cm)
				default:
					continue
				}
			}
		}
	}
	return bundle, nil
}

func getObject(obj map[string]interface{}, newObj interface{}) error {
	o, err := yamlv2.Marshal(&obj)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(o, newObj)
}
