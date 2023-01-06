package controller

import (
	"context"
	"github.com/stretchr/testify/assert"
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
	"testing"
	"time"
)

func fakeControllerSetup(t *testing.T, objects ...runtime.Object) (func(), kubernetes.Interface, string, context.Context, *Controller) {
	client := fake.NewSimpleClientset(objects...)

	var basePath string
	if dir, err := os.MkdirTemp("", ""); err != nil {
		t.Fatalf("could not create temporary file")
	} else {
		basePath = path.Join(dir, "kubedump-test")
	}

	ctx, cancel := context.WithCancel(context.Background())

	filterExpression, _ := filter.Parse("")

	opts := Options{
		BasePath:      basePath,
		Filter:        filterExpression,
		ParentContext: ctx,
		//Logger:        kubedump.NewLogger(kubedump.WithLevel(zap.NewAtomicLevelAt(zap.DebugLevel))),
		Logger:         kubedump.NewLogger(),
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

		if err := os.RemoveAll(basePath); err != nil {
			t.Errorf("failed to delete temporary test directory '%s': %s", basePath, err)
		}
	}

	return teardown, client, basePath, ctx, controller
}

func TestPodEvent(t *testing.T) {
	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &tests.SamplePod)

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

	err := controller.Start(5)
	assert.NoError(t, err)

	if _, err := client.EventsV1().Events(tests.ResourceNamespace).Create(ctx, handledEvent.Resource.(*apieventsv1.Event), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create resource '%s': %s", handledEvent.String(), err)
	}

	tests.WaitForResources(t, basePath, ctx, handledPod)

	err = controller.Stop()
	assert.NoError(t, err)

	tests.AssertResource(t, basePath, handledPod, true)
}

func TestSelector(t *testing.T) {
	handledJob, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apibatchv1.Job{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-job",
			Namespace: tests.ResourceNamespace,
			UID:       "sample-job-uid",
		},
		Spec: apibatchv1.JobSpec{
			Selector: &apimetav1.LabelSelector{
				MatchLabels: map[string]string{"controller-uid": "sample-job-uid"},
			},
			Template: apicorev1.PodTemplateSpec{},
		},
	})

	handledPod, _ := kubedump.NewHandledResource(kubedump.HandleAdd, &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      "sample-job-xxxx",
			Namespace: tests.ResourceNamespace,
			Labels: map[string]string{
				"job-name":       handledJob.GetName(),
				"controller-uid": string(handledJob.GetUID()),
			},
		},
		Spec: apicorev1.PodSpec{},
	})

	teardown, client, basePath, ctx, controller := fakeControllerSetup(t, handledJob.Resource.(*apibatchv1.Job))
	defer teardown()

	err := controller.Start(5)
	assert.NoError(t, err)

	if _, err := client.CoreV1().Pods(tests.ResourceNamespace).Create(ctx, handledPod.Resource.(*apicorev1.Pod), apimetav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create resource '%s': %s", handledPod, err)
	}

	tests.WaitForResources(t, basePath, ctx, handledPod, handledJob)

	tests.AssertResource(t, basePath, handledJob, false)
	tests.AssertResource(t, basePath, handledPod, false)
	tests.AssertResourceIsLinked(t, basePath, handledJob, handledPod)

	err = controller.Stop()
	assert.NoError(t, err)
}
