package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/joshmeranda/kubedump/pkg/filter"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// todo: decrease syn-log frequency
const (
	ResyncTime = time.Second * 5
)

type Options struct {
	BasePath       string
	ParentContext  context.Context
	Logger         *slog.Logger
	LogSyncTimeout time.Duration
	Resources      []schema.GroupVersionResource
}

// todo: move job handling into job.go
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
	kubeclientset kubernetes.Interface,
	dynamicclientset dynamic.Interface,
	opts Options,
) (*Controller, error) {
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

		informerFactory: dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicclientset, ResyncTime, apicorev1.NamespaceAll, nil),
		stopChan:        nil,

		logStreams: make(map[string]Stream),

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		ctx:    ctx,
		cancel: cancel,

		store: NewStore(),

		informers: make(map[string]cache.SharedIndexInformer),
	}

	if len(opts.Resources) == 0 {
		opts.Logger.Warn("no resources were specified")
	}

	for _, resource := range opts.Resources {
		controller.Logger.Debug(fmt.Sprintf("registering resource '%s'", resource.Resource))

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

		if _, err := informer.AddEventHandler(handler); err != nil {
			controller.Logger.Error(fmt.Sprintf("could not add event handler for resource '%s': %s", resource.Resource, err))
		} else {
			controller.informers[fmt.Sprintf("%s:%s:%s", resource.Group, resource.Version, resource.Resource)] = informer
		}
	}

	eventInformer := informers.NewSharedInformerFactory(kubeclientset, ResyncTime).Events().V1().Events().Informer()
	_, err := eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleEvent,
	})
	if err != nil {
		return nil, fmt.Errorf("could not add event handler: %w", err)
	}

	controller.informers["events.k8s.io/v1"] = eventInformer

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
				controller.Logger.Debug(fmt.Sprintf("error syncing container '%s': %s", id, err))
			} else {
				controller.Logger.Error(fmt.Sprintf("error syncing container '%s': %s", id, err))
			}
		} else {
			controller.Logger.Debug(fmt.Sprintf("synced logs for container '%s'", id))
		}
	}

	controller.logStreamsMu.Unlock()

	time.Sleep(time.Second * 1)
	controller.workQueue.AddRateLimited(NewJob(controller.ctx, JobNameSyncLogs, func() {
		controller.syncLogStreams()
	}))
}

func (controller *Controller) processNextWorkItem() bool {
	obj, _ := controller.workQueue.Get()

	if obj == nil {
		return true
	}

	job, ok := obj.(Job)

	controller.Logger.Debug(fmt.Sprintf("processing next work item '%s'", job.name))

	if !ok {
		controller.Logger.Error(fmt.Sprintf("could not understand worker function of type '%T'", obj))
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

	controller.Logger.Info("starting controller")

	controller.informerFactory.Start(controller.stopChan)

	controller.startTime = time.Now().UTC()

	controller.workerWaitGroup.Add(nWorkers)

	for i := 0; i < nWorkers; i++ {
		n := i
		go func() {
			controller.workerWaitGroup.Done()

			controller.Logger.Debug(fmt.Sprintf("starting worker #%d", n))

			for !(controller.workQueue.ShuttingDown() && controller.workQueue.Len() == 0) {
				controller.processNextWorkItem()
			}

			controller.Logger.Debug(fmt.Sprintf("stopping worker #%d", n))

			controller.workerWaitGroup.Done()
		}()
	}

	// we add nWorker back to the wait group to allow for waiting for workers during stop
	controller.workerWaitGroup.Wait()
	controller.workerWaitGroup.Add(nWorkers)

	controller.workQueue.AddRateLimited(NewJob(controller.ctx, JobNameSyncLogs, func() {
		controller.syncLogStreams()
	}))

	// todo: list and load existing resources on startup

	return nil
}

func (controller *Controller) Stop() error {
	if controller.stopChan == nil {
		return fmt.Errorf("controller was not running")
	}
	controller.Logger.Info("Stopping controller")

	close(controller.stopChan)
	controller.stopChan = nil

	controller.workQueue.ShutDownWithDrain()

	controller.workerWaitGroup.Wait()

	controller.filterExpr = nil

	return nil
}
