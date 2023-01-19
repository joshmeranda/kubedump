package controller

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubedump "kubedump/pkg"
	"testing"
)

func TestStorage(t *testing.T) {
	job := &apibatchv1.Job{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: "default",
			Name:      "sample-job",
			UID:       "sample-job-uid",
		},
		Spec: apibatchv1.JobSpec{
			Selector: &apimetav1.LabelSelector{
				MatchLabels:      map[string]string{"controller-uid": "sample-job-uid"},
				MatchExpressions: nil,
			},
		},
	}

	pod := &apicorev1.Pod{
		ObjectMeta: apimetav1.ObjectMeta{
			Namespace: "default",
			Name:      "sample-job-xxxxx",
			UID:       "sample-job-xxxxx-uid",
			Labels: map[string]string{
				"controller-uid": "sample-job-uid",
				"job-name":       "sample-job",
			},
		},
	}

	handledJob, err := kubedump.NewHandledResource(job)
	assert.NoError(t, err)

	handledPod, err := kubedump.NewHandledResource(pod)
	assert.NoError(t, err)

	store := NewStore()

	matcher, err := selectorFromHandled(handledJob)
	assert.NoError(t, err)
	assert.NotNil(t, matcher)

	err = store.AddResource(handledJob, matcher)
	assert.NoError(t, err)

	matchers, err := store.GetResources(handledPod)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(matchers))
	assert.Equal(t, fmt.Sprintf("%s/%s/%s", handledJob.Kind, handledJob.GetNamespace(), handledJob.GetName()), matchers[0].String())
}
