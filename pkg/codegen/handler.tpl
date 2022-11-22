// The code in this file was generated using ./pkg/codegen, do not modify it directly

package controller

import (
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
	"time"
	{{ .AdditionalImports -}}
)

func mostRecent{{ .TypeName }}ConditionTime(conditions []{{ .ConditionTypePath }}) time.Time {
	if len(conditions) == 0 {
		// if there are no conditions we'd rather take it than not
		return time.Now().UTC()
	}

	var t time.Time

	for _, condition := range conditions {

		if utc := condition.LastTransitionTime.UTC(); utc.After(t) {
			t = utc
		}
	}

	return t
}

type {{ .TypeName }}Handler struct {
	// will be inherited from parent controller
	opts      Options
	workQueue workqueue.RateLimitingInterface
}

func New{{ .TypeName }}Handler(opts Options, workQueue workqueue.RateLimitingInterface) *{{ .TypeName }}Handler {
	return &{{ .TypeName }}Handler{
		opts:      opts,
		workQueue: workQueue,
	}
}

func (handler *{{ .TypeName }}Handler) handleFunc(obj interface{}, isAdd bool) {
	resource, ok := obj.(*{{ .TypePath }})

	if !ok {
		logrus.Errorf("could not coerce object to {{ .TypeName }}")
		return
	}

	if !handler.opts.Filter.Matches(resource) || handler.opts.StartTime.After(mostRecent{{ .TypeName }}ConditionTime(resource.Status.Conditions)) {
		return
	}

	if isAdd {
		linkResourceOwners(handler.opts.ParentPath, "{{ .TypeName }}", resource)
	}

	handler.workQueue.AddRateLimited(NewJob(func() {
		if err := dumpResourceDescription(handler.opts.ParentPath, "{{ .TypeName }}", resource); err != nil {
			logrus.WithFields(resourceFields(resource)).Errorf("could not dump {{ .TypeName }} description: %s", err)
		}
	}))
}

func (handler *{{ .TypeName }}Handler) OnAdd(obj interface{}) {
	handler.handleFunc(obj, true)
}

func (handler *{{ .TypeName }}Handler) OnUpdate(_ interface{}, obj interface{}) {
	handler.handleFunc(obj, false)
}

func (handler *{{ .TypeName }}Handler) OnDelete(obj interface{}) {
	handler.handleFunc(obj, false)
}
