package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/joshmeranda/kubedump/pkg/filter"
	"github.com/joshmeranda/kubedump/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apieventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var testControllerResources = []schema.GroupVersionResource{
	// {Group: "", Version: "v1", Resource: "bindings"},
	{Group: "", Version: "v1", Resource: "configmaps"},
	{Group: "", Version: "v1", Resource: "events"},
	{Group: "", Version: "v1", Resource: "podtemplates"},
	{Group: "", Version: "v1", Resource: "pods"},
	{Group: "", Version: "v1", Resource: "secrets"},
	{Group: "", Version: "v1", Resource: "replicationcontrollers"},
	{Group: "", Version: "v1", Resource: "endpoints"},
	{Group: "", Version: "v1", Resource: "resourcequotas"},
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	{Group: "", Version: "v1", Resource: "limitranges"},
	{Group: "", Version: "v1", Resource: "serviceaccounts"},
	{Group: "", Version: "v1", Resource: "services"},
	// {Group: "apps", Version: "v1", Resource: "statefulsets"},
	// {Group: "apps", Version: "v1", Resource: "daemonsets"},
	// {Group: "apps", Version: "v1", Resource: "replicasets"},
	// {Group: "apps", Version: "v1", Resource: "deployments"},
	// {Group: "apps", Version: "v1", Resource: "controllerrevisions"},
	// {Group: "events.k8s.io", Version: "v1", Resource: "events"},
	// {Group: "authorization.k8s.io", Version: "v1", Resource: "localsubjectaccessreviews"},
	// {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	// {Group: "batch", Version: "v1", Resource: "jobs"},
	// {Group: "batch", Version: "v1", Resource: "cronjobs"},
	// {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	// {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	// {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
	// {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	// {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	// {Group: "storage.k8s.io", Version: "v1", Resource: "csistoragecapacities"},
	// {Group: "coordination.k8s.io", Version: "v1", Resource: "leases"},
	// {Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"},
	// {Group: "helm.cattle.io", Version: "v1", Resource: "helmcharts"},
	// {Group: "helm.cattle.io", Version: "v1", Resource: "helmchartconfigs"},
	// {Group: "k3s.cattle.io", Version: "v1", Resource: "addons"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "traefikservices"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "tlsstores"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "middlewares"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "ingressroutes"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "serverstransports"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "middlewaretcps"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "tlsoptions"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "ingressroutetcps"},
	// {Group: "traefik.containo.us", Version: "v1alpha1", Resource: "ingressrouteudps"},
	// {Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"},
}

func resourceToHandled[T any](t *testing.T, obj T) (kubedump.Resource, T) {
	data, err := json.Marshal(obj)
	require.NoError(t, err)

	var u unstructured.Unstructured
	require.NoError(t, json.Unmarshal(data, &u))

	resource := kubedump.NewResourceBuilder().FromUnstructured(&u).Build()

	return resource, obj
}

func filterForResource(t *testing.T, resource kubedump.Resource) filter.Expression {
	s := fmt.Sprintf("%s %s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
	expr, err := filter.Parse(s)
	if err != nil {
		t.Fatalf("failed to parse expression '%s': %s", s, err)
	}

	return expr
}

func fakeControllerSetup(t *testing.T, objects ...runtime.Object) (func(), kubernetes.Interface, string, context.Context, *Controller) {
	client := fake.NewSimpleClientset(objects...)

	scheme := runtime.NewScheme()
	if err := apicorev1.AddToScheme(scheme); err != nil {
		t.Errorf("could not add sceheme: %s", err)
	}

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, objects...)

	basePath := path.Join(t.TempDir(), "kubedump-test")
	logFilePath := path.Join(basePath, "kubedump.log")

	if err := createPathParents(logFilePath); err != nil {
		t.Fatalf("could not create log file '%s': %s", logFilePath, err)
	}

	if f, err := os.Create(logFilePath); err != nil {
		t.Fatalf("could not create log file '%s': %s", logFilePath, err)
	} else {
		f.Close()
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, cancel := context.WithCancel(context.Background())

	opts := Options{
		BasePath:       basePath,
		ParentContext:  ctx,
		Logger:         logger,
		LogSyncTimeout: time.Second,
		Resources:      testControllerResources,
	}

	controller, _ := NewController(client, dynamicClient, opts)

	teardown := func() {
		cancel()

		if t.Failed() {
			dumpDir := t.Name() + ".dump"
			t.Logf("copying dump directory int '%s' for failed test", dumpDir)

			if err := os.RemoveAll(dumpDir); err != nil && !os.IsNotExist(err) {
				t.Errorf("error removing existing test dump: %s", err)
			}

			if err := tests.CopyTree(basePath, dumpDir); err != nil {
				t.Errorf("%s", err)
			}
		}
	}

	return teardown, client, basePath, ctx, controller
}

func TestEvent(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
	})

	handledEvent, event := resourceToHandled(t, &apieventsv1.Event{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Event",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "sample-pod-event",
		},
		EventTime: apimetav1.MicroTime{
			Time: time.Now().Add(time.Hour),
		},
		ReportingController: "some-controller",
		ReportingInstance:   "some-instance",
		Action:              "update",
		Reason:              "something happened",
		Regarding: apicorev1.ObjectReference{
			Kind:            "Pod",
			Namespace:       tests.ResourceNamespace,
			Name:            pod.GetName(),
			UID:             pod.GetUID(),
			APIVersion:      pod.APIVersion,
			ResourceVersion: pod.GetResourceVersion(),
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, pod)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledPod))
	assert.NoError(t, err)

	if _, err := client.EventsV1().Events(tests.ResourceNamespace).Create(ctx, event, apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create resource '%s': %s", handledEvent.String(), err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	eventFile := kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).WithFileName(handledPod.GetName() + ".yaml").Build()
	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, eventFile); err != nil {
		t.Fatalf("failed witing for path: ")
	}

	err = controller.Stop()
	assert.NoError(t, err)
}

func TestLogs(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, pod)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledPod))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	time.Sleep(time.Millisecond * 10)

	err = controller.Stop()
	assert.NoError(t, err)

	logFile := kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).WithFileName(handledPod.GetName() + ".log").Build()
	data, err := os.ReadFile(logFile)
	assert.GreaterOrEqual(t, 1, strings.Count(string(data), "fake logs"))
	assert.NoError(t, err)
}

func TestPod(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, pod)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledPod))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
}

func TestService(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"test": "replicaset",
			},
			UID: "sample-pod-uid",
		},
	})

	handledService, service := resourceToHandled(t, &apicorev1.Service{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-service",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-service-uid",
		},
		Spec: apicorev1.ServiceSpec{
			Selector: handledPod.GetLabels(),
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, service)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledService))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledService).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, pod, apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledService, false)
	tests.AssertResourceIsLinked(t, basePath, handledService, handledPod)
}

func TestJob(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"controller-uid": "sample-job",
			},
			UID: "sample-pod-uid",
		},
	})

	handledJob, job := resourceToHandled(t, &apibatchv1.Job{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Job",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-job",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-job-uid",
		},
		Spec: apibatchv1.JobSpec{
			Selector: &apimetav1.LabelSelector{
				MatchLabels:      handledPod.GetLabels(),
				MatchExpressions: nil,
			},
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, job)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledJob))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledJob).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, pod, apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledJob, false)
	tests.AssertResourceIsLinked(t, basePath, handledJob, handledPod)
}

func TestReplicaSet(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"test": "replicaset",
			},
			UID: "sample-pod-uid",
		},
	})

	handledReplicaSet, replicaset := resourceToHandled(t, &apiappsv1.ReplicaSet{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "ReplicaSet",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-replica-set",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-replicaset-uid",
		},
		Spec: apiappsv1.ReplicaSetSpec{
			Selector: &apimetav1.LabelSelector{
				MatchLabels:      handledPod.GetLabels(),
				MatchExpressions: nil,
			},
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, replicaset)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledReplicaSet))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledReplicaSet).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, pod, apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledReplicaSet, false)
	tests.AssertResourceIsLinked(t, basePath, handledReplicaSet, handledPod)
}

func TestDeployment(t *testing.T) {
	handledPod, pod := resourceToHandled(t, &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"test": "deployment",
			},
			UID: "sample-pod-uid",
		},
	})

	handledDeployment, deployment := resourceToHandled(t, &apiappsv1.Deployment{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-deployment",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-deployment-uid",
		},
		Spec: apiappsv1.DeploymentSpec{
			Selector: &apimetav1.LabelSelector{
				MatchLabels:      handledPod.GetLabels(),
				MatchExpressions: nil,
			},
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, deployment)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledDeployment))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledDeployment).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, pod, apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledPod).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledDeployment, false)
	tests.AssertResourceIsLinked(t, basePath, handledDeployment, handledPod)
}

func TestConfigMap(t *testing.T) {
	handledConfigMap, configmap := resourceToHandled(t, &apicorev1.ConfigMap{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-configmap",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-configmap-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, configmap)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledConfigMap))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledConfigMap).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledConfigMap)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledConfigMap, false)
}

func TestSecret(t *testing.T) {
	handledSecret, secret := resourceToHandled(t, &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-secret",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-secret-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, secret)
	defer teardown()

	err := controller.Start(tests.UnitNWorkers, filterForResource(t, handledSecret))
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, kubedump.NewResourcePathBuilder().WithBase(basePath).WithResource(handledSecret).Build()); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledSecret)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledSecret, false)
}
