package controller

import (
	"encoding/json"
	"fmt"
	"testing"

	kubedump "github.com/joshmeranda/kubedump/pkg"
	"github.com/stretchr/testify/assert"
	apibatchv1 "k8s.io/api/batch/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func destructureObject(v any) *unstructured.Unstructured {
	data, err := json.Marshal(v)
	if err != nil {
		panic("could not marshal object: " + err.Error())
	}

	u := &unstructured.Unstructured{}
	if err := json.Unmarshal(data, &u); err != nil {
		panic("could not unmarshal object: " + err.Error())
	}

	return u
}

func TestStorage(t *testing.T) {
	rawJob := &apibatchv1.Job{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Job",
		},
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

	rawPod := &apicorev1.Pod{
		TypeMeta: apimetav1.TypeMeta{
			Kind: "Pod",
		},
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

	job := kubedump.NewResourceBuilder().
		FromObject(rawJob.ObjectMeta).
		WithKind("Job").
		Build()

	pod := kubedump.NewResourceBuilder().
		FromObject(rawPod.ObjectMeta).
		WithKind("Pod").
		Build()

	store := NewStore()

	matcher, err := selectorFromUnstructured(destructureObject(rawJob))
	assert.NoError(t, err)
	assert.NotNil(t, matcher)

	err = store.AddResource(job, matcher)
	assert.NoError(t, err)

	matchers, err := store.GetResources(pod)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(matchers))
	assert.Equal(t, fmt.Sprintf("%s/%s", job.GetKind(), job.GetName()), matchers[0].String())
}
