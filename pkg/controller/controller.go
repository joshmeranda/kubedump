package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/joshmeranda/kubedump/pkg/filter"
	"github.com/samber/lo"
	"go.uber.org/zap"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	FakeHost = "FAKE"
)

// todo: maybe create a InformerGroup type here

var defaultResources = []schema.GroupVersionResource{
	// {
	// 	Group:    "events.k8s.io",
	// 	Version:  "v1",
	// 	Resource: "event",
	// },
	{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	},
	// {
	// 	Group:    "core",
	// 	Version:  "v1",
	// 	Resource: "pod",
	// },
	// {
	// 	Group:    "core",
	// 	Version:  "v1",
	// 	Resource: "service",
	// },
	// {
	// 	Group:    "core",
	// 	Version:  "v1",
	// 	Resource: "secret",
	// },
	// {
	// 	Group:    "core",
	// 	Version:  "v1",
	// 	Resource: "configmap",
	// },
	// {
	// 	Group:    "batch",
	// 	Version:  "v1",
	// 	Resource: "job",
	// },
	// {
	// 	Group:    "apps",
	// 	Version:  "v1",
	// 	Resource: "replicaset",
	// },
	// {
	// 	Group:    "apps",
	// 	Version:  "v1",
	// 	Resource: "deployments",
	// },
}

type Job struct {
	id  uuid.UUID
	ctx context.Context
	fn  *func()
}

func NewJob(ctx context.Context, fn func()) Job {
	return Job{
		id:  uuid.New(),
		ctx: ctx,
		fn:  &fn,
	}
}

type Options struct {
	BasePath       string
	ParentContext  context.Context
	Logger         *zap.SugaredLogger
	LogSyncTimeout time.Duration

	// todo: this is a bad way to inject a fake client for testing. Needed so we can build a dynamic client and normal client using the same config.
	FakeClient *fake.Clientset
}

type Controller struct {
	Options

	kubeclientset kubernetes.Interface
	startTime     time.Time

	filterExpr filter.Expression

	informerFactory dynamicinformer.DynamicSharedInformerFactory
	stopChan        chan struct{}

	workerWaitGroup sync.WaitGroup

	// logStreams is a store of logStreams mapped to a unique identifier for the associated container.
	logStreams   map[string]Stream
	logStreamsMu sync.Mutex

	workQueue workqueue.RateLimitingInterface

	ctx    context.Context
	cancel context.CancelFunc

	store     Store
	informers map[string]cache.SharedIndexInformer
}

func NewController(
	config *rest.Config,
	opts Options,
) (*Controller, error) {
	var err error
	var kubeclientset kubernetes.Interface

	if opts.FakeClient != nil {
		kubeclientset = opts.FakeClient
	} else if kubeclientset == nil {
		kubeclientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("could not create clientset from given config: %w", err)
		}
	}

	dynamicclientset, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create dynamic clientset from given config: %w", err)
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if opts.ParentContext != nil {
		ctx, cancel = context.WithCancel(opts.ParentContext)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	controller := &Controller{
		Options:       opts,
		kubeclientset: kubeclientset,

		informerFactory: dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicclientset, time.Second*5, apicorev1.NamespaceAll, nil),
		stopChan:        nil,

		logStreams: make(map[string]Stream),

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		ctx:    ctx,
		cancel: cancel,

		store: NewStore(),

		informers: make(map[string]cache.SharedIndexInformer),
	}

	for _, resource := range defaultResources {
		handler := cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				controller.onAdd(resource, obj)
			},
			UpdateFunc: func(_ any, new any) {
				controller.onUpdate(resource, new)
			},
			DeleteFunc: func(obj any) {
				controller.onDelete(resource, obj)
			},
		}
		informer := controller.informerFactory.ForResource(resource).Informer()
		informer.AddEventHandler(handler)
		controller.informers[fmt.Sprintf("%s:%s:%s", resource.Group, resource.Version, resource.Resource)] = informer
	}

	return controller, nil
}

func (controller *Controller) syncLogStreams() {
	select {
	case <-controller.stopChan:
		return
	case <-controller.ctx.Done():
		return
	default:
	}

	controller.logStreamsMu.Lock()

	for id, stream := range controller.logStreams {
		if err := stream.Sync(); err != nil {
			if strings.Contains(err.Error(), "ContainerCreating") {
				controller.Logger.Debugf("error syncing container '%s': %s", id, err)
			} else {
				controller.Logger.Errorf("error syncing container '%s': %s", id, err)
			}
		} else {
			controller.Logger.Debugf("synced logs for container '%s'", id)
		}
	}

	controller.logStreamsMu.Unlock()

	controller.workQueue.AddRateLimited(NewJob(controller.ctx, func() {
		controller.syncLogStreams()
	}))
}

func (controller *Controller) processNextWorkItem() bool {
	obj, _ := controller.workQueue.Get()

	if obj == nil {
		return true
	}

	job, ok := obj.(Job)

	controller.Logger.Debugf("processing next work item '%s'", job.id)

	if !ok {
		controller.Logger.Errorf("could not understand worker function of type '%T'", obj)
		controller.workQueue.Forget(obj)
		return false
	}

	// todo: this *could* block which ain't good
	(*job.fn)()
	controller.workQueue.Done(obj)

	return true
}

func (controller *Controller) Start(nWorkers int, expr filter.Expression) error {
	if controller.stopChan != nil {
		return fmt.Errorf("controller is already running")
	}
	defer runtime.HandleCrash()

	controller.filterExpr = expr

	controller.stopChan = make(chan struct{})

	controller.Logger.Infof("starting controller")

	controller.informerFactory.Start(controller.stopChan)

	controller.Logger.Infof("waiting for informer caches to sync")
	informersHaveSynced := lo.MapToSlice(controller.informers, func(_ string, informer cache.SharedIndexInformer) cache.InformerSynced {
		return informer.HasSynced
	})
	if ok := cache.WaitForNamedCacheSync("kubedump", controller.stopChan, informersHaveSynced...); !ok {
		return fmt.Errorf("could not wait for caches to sync")
	}

	controller.Logger.Infof("caches synced")

	controller.startTime = time.Now().UTC()

	controller.workerWaitGroup.Add(nWorkers)

	for i := 0; i < nWorkers; i++ {
		n := i
		go func() {
			controller.workerWaitGroup.Done()

			// controller.Logger.Debugf("starting worker #%d", n)
			controller.Logger.Infof("starting worker #%d", n)

			// workerLoop:
			for !(controller.workQueue.ShuttingDown() && controller.workQueue.Len() == 0) {
				// select {
				// case <-controller.ctx.Done():
				// 	break workerLoop
				// default:
				// }

				controller.processNextWorkItem()
			}

			controller.Logger.Debugf("stopping worker #%d", n)

			controller.workerWaitGroup.Done()
		}()
	}

	// we add nWorker back to the wait group to allow for waiting for workers during stop
	controller.workerWaitGroup.Wait()
	controller.workerWaitGroup.Add(nWorkers)

	controller.workQueue.AddRateLimited(NewJob(controller.ctx, func() {
		controller.syncLogStreams()
	}))

	controller.Logger.Infof("Started controller")

	return nil
}

func (controller *Controller) Stop() error {
	if controller.stopChan == nil {
		return fmt.Errorf("controller was not running")
	}
	controller.Logger.Infof("Stopping controller")

	close(controller.stopChan)
	controller.stopChan = nil

	controller.workQueue.ShutDownWithDrain()

	controller.workerWaitGroup.Wait()

	controller.filterExpr = nil

	return nil
}
