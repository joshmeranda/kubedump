package controller

import (
	"context"
	"github.com/stretchr/testify/assert"
	apiappsv1 "k8s.io/api/apps/v1"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apieventsv1 "k8s.io/api/events/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kubedump "kubedump/pkg"
	"kubedump/pkg/filter"
	"kubedump/tests"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func fakeControllerSetup(t *testing.T, objects ...runtime.Object) (func(), kubernetes.Interface, string, context.Context, *Controller) {
	client := fake.NewSimpleClientset(objects...)

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

	ctx, cancel := context.WithCancel(context.Background())

	filterExpression, _ := filter.Parse("")

	loggerOptions := []kubedump.LoggerOption{
		//kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel)),
		kubedump.WithPaths(logFilePath),
	}

	opts := Options{
		BasePath:       basePath,
		Filter:         filterExpression,
		ParentContext:  ctx,
		Logger:         kubedump.NewLogger(loggerOptions...),
		LogSyncTimeout: time.Second,
	}

	controller, _ := NewController(client, opts)

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
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
	})

	handledEvent, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apieventsv1.Event{
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
			Name:            handledPod.GetName(),
			UID:             handledPod.GetUID(),
			APIVersion:      handledPod.APIVersion,
			ResourceVersion: handledPod.GetResourceVersion(),
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, handledPod.Resource.(*apicorev1.Pod))
	defer teardown()

	t.Log(basePath)

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if _, err := client.EventsV1().Events(tests.ResourceNamespace).Create(ctx, handledEvent.Resource.(*apieventsv1.Event), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create resource '%s': %s", handledEvent.String(), err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	eventFile := resourceFilePath(basePath, handledPod.Kind, handledPod, handledPod.GetName()+".events")
	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, eventFile); err != nil {
		t.Fatalf("failed witing for path: ")
	}

	err = controller.Stop()
	assert.NoError(t, err)
}

func TestLogs(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, handledPod.Resource.(*apicorev1.Pod))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	time.Sleep(time.Millisecond * 10)

	err = controller.Stop()
	assert.NoError(t, err)

	logFile := resourceFilePath(basePath, handledPod.Kind, handledPod.Resource.(*apicorev1.Pod), handledPod.GetName()+".log")
	data, err := os.ReadFile(logFile)

	assert.GreaterOrEqual(t, 1, strings.Count(string(data), "fake logs"))
}

func TestPod(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, handledPod.Resource.(*apicorev1.Pod))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
}

func TestPodWithConfigMap(t *testing.T) {
	handledConfigMap, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-configmap",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-configmap-uid",
		},
	})

	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
		Spec: apicorev1.PodSpec{
			Volumes: []apicorev1.Volume{
				{
					Name: "sample-configmap",
					VolumeSource: apicorev1.VolumeSource{
						ConfigMap: &apicorev1.ConfigMapVolumeSource{
							LocalObjectReference: apicorev1.LocalObjectReference{
								Name: handledConfigMap.GetName(),
							},
						},
					},
				},
			},
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, handledConfigMap.Resource.(*apicorev1.ConfigMap), handledPod.Resource.(*apicorev1.Pod))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledConfigMap.Kind, handledConfigMap)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledConfigMap, false)
	tests.AssertResourceIsLinked(t, basePath, handledPod, handledConfigMap)
}

func TestPodWithSecret(t *testing.T) {
	handledSecret, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-secret",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-secret-uid",
		},
	})

	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-pod-uid",
		},
		Spec: apicorev1.PodSpec{
			Volumes: []apicorev1.Volume{
				{
					Name: "sample-configmap",
					VolumeSource: apicorev1.VolumeSource{
						Secret: &apicorev1.SecretVolumeSource{
							SecretName: handledSecret.GetName(),
						},
					},
				},
			},
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, handledPod.Resource.(*apicorev1.Pod), handledSecret.Resource.(*apicorev1.Secret))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledSecret.Kind, handledSecret)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)
}

func TestService(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"test": "replicaset",
			},
			UID: "sample-pod-uid",
		},
	})

	handledService, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Service{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-replica-set",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-service-uid",
		},
		Spec: apicorev1.ServiceSpec{
			Selector: handledPod.GetLabels(),
		},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, handledService.Resource.(*apicorev1.Service))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledService.Kind, handledService)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, handledPod.Resource.(*apicorev1.Pod), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledService, false)
	tests.AssertResourceIsLinked(t, basePath, handledService, handledPod)
}

func TestJob(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"controller-uid": "sample-job",
			},
			UID: "sample-pod-uid",
		},
	})

	handledJob, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apibatchv1.Job{
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

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, handledJob.Resource.(*apibatchv1.Job))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledJob.Kind, handledJob)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, handledPod.Resource.(*apicorev1.Pod), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledJob, false)
	tests.AssertResourceIsLinked(t, basePath, handledJob, handledPod)
}

func TestReplicaSet(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"test": "replicaset",
			},
			UID: "sample-pod-uid",
		},
	})

	handledReplicaSet, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apiappsv1.ReplicaSet{
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

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, handledReplicaSet.Resource.(*apiappsv1.ReplicaSet))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledReplicaSet.Kind, handledReplicaSet)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, handledPod.Resource.(*apicorev1.Pod), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledReplicaSet, false)
	tests.AssertResourceIsLinked(t, basePath, handledReplicaSet, handledPod)
}

func TestDeployment(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"test": "deployment",
			},
			UID: "sample-pod-uid",
		},
	})

	handledDeployment, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apiappsv1.Deployment{
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

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, handledDeployment.Resource.(*apiappsv1.Deployment))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledDeployment.Kind, handledDeployment)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	if _, err = client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, handledPod.Resource.(*apicorev1.Pod), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("erro creating resource %s: %s", handledPod, err)
	}

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledPod.Kind, handledPod)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledPod)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResource(t, basePath, handledDeployment, false)
	tests.AssertResourceIsLinked(t, basePath, handledDeployment, handledPod)
}

func TestConfigMap(t *testing.T) {
	handledConfigMap, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.ConfigMap{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-configmap",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-configmap-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, handledConfigMap.Resource.(*apicorev1.ConfigMap))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledConfigMap.Kind, handledConfigMap)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledConfigMap)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledConfigMap, false)
}

func TestSecret(t *testing.T) {
	handledSecret, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-secret",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-secret-uid",
		},
	})

	teardown, _, basePath, ctx, controller := fakeControllerSetup(t, handledSecret.Resource.(*apicorev1.Secret))
	defer teardown()

	err := controller.Start(tests.NWorkers)
	assert.NoError(t, err)

	if err := tests.WaitForPath(ctx, tests.TestWaitDuration, resourceDirPath(basePath, handledSecret.Kind, handledSecret)); err != nil {
		t.Fatalf("error waiting for resource path: %s", handledSecret)
	}

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledSecret, false)
}
