package controller

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	informersappsv1 "k8s.io/client-go/informers/apps/v1"
	informersbatchv1 "k8s.io/client-go/informers/batch/v1"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubedump/pkg/filter"
	"strings"
	"sync"
	"time"
)

type Job struct {
	id uuid.UUID
	fn *func()
}

func NewJob(fn func()) Job {
	return Job{
		id: uuid.New(),
		fn: &fn,
	}
}

type Options struct {
	ParentPath     string
	Filter         filter.Expression
	ParentContext  context.Context
	Logger         *zap.SugaredLogger
	LogSyncTimeout time.Duration
}

type Controller struct {
	Options

	kubeclientset kubernetes.Interface
	startTime     time.Time

	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}

	sieve Sieve

	// logStreams is a store of logStreams mapped to a unique identifier for the associated container.
	logStreams   map[string]Stream
	logStreamsMu sync.Mutex

	workQueue workqueue.RateLimitingInterface

	ctx    context.Context
	cancel context.CancelFunc

	store Store

	informersSynced []cache.InformerSynced

	podInformer        informerscorev1.PodInformer
	serviceInformer    informerscorev1.ServiceInformer
	configMapInformer  informerscorev1.ConfigMapInformer
	jobInformer        informersbatchv1.JobInformer
	replicasetInformer informersappsv1.ReplicaSetInformer
	deploymentInformer informersappsv1.DeploymentInformer
}

func NewController(
	kubeclientset kubernetes.Interface,
	opts Options,
) (*Controller, error) {
	informerFactory := informers.NewSharedInformerFactory(kubeclientset, time.Second*5)

	sieve, err := NewSieve(opts.Filter)
	if err != nil {
		return nil, fmt.Errorf("could not create resource filter: %w", err)
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

		informerFactory: informerFactory,
		stopChan:        nil,

		sieve: sieve,

		logStreams: make(map[string]Stream),

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		ctx:    ctx,
		cancel: cancel,

		store: NewStore(),

		podInformer:        informerFactory.Core().V1().Pods(),
		serviceInformer:    informerFactory.Core().V1().Services(),
		configMapInformer:  informerFactory.Core().V1().ConfigMaps(),
		jobInformer:        informerFactory.Batch().V1().Jobs(),
		replicasetInformer: informerFactory.Apps().V1().ReplicaSets(),
		deploymentInformer: informerFactory.Apps().V1().Deployments(),
	}

	eventInformer := informerFactory.Events().V1().Events()

	controller.informersSynced = []cache.InformerSynced{
		eventInformer.Informer().HasSynced,

		controller.podInformer.Informer().HasSynced,
		controller.serviceInformer.Informer().HasSynced,
		controller.configMapInformer.Informer().HasSynced,
		controller.jobInformer.Informer().HasSynced,
		controller.replicasetInformer.Informer().HasSynced,
		controller.deploymentInformer.Informer().HasSynced,
	}

	handler := cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.onAdd,
		UpdateFunc: controller.onUpdate,
		DeleteFunc: controller.onDelete,
	}

	eventInformer.Informer().AddEventHandler(handler)

	controller.podInformer.Informer().AddEventHandler(handler)
	controller.serviceInformer.Informer().AddEventHandler(handler)
	controller.configMapInformer.Informer().AddEventHandler(handler)

	controller.jobInformer.Informer().AddEventHandler(handler)

	controller.replicasetInformer.Informer().AddEventHandler(handler)
	controller.deploymentInformer.Informer().AddEventHandler(handler)

	return controller, nil
}

func (controller *Controller) syncLogStreams() {
	select {
	case <-controller.stopChan:
		return
	default:
	}

	controller.logStreamsMu.Lock()
	controller.Logger.Infof("syncing container logs")

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

	controller.workQueue.AddRateLimited(NewJob(func() {
		controller.syncLogStreams()
	}))
}

func (controller *Controller) processNextWorkItem() bool {
	obj, _ := controller.workQueue.Get()

	if obj == nil {
		return true
	}

	job, ok := obj.(Job)

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

func (controller *Controller) Sync() {
	// doing nothing
}

func (controller *Controller) Start(nWorkers int) error {
	if controller.stopChan != nil {
		return fmt.Errorf("controller is already running")
	}
	defer runtime.HandleCrash()

	controller.stopChan = make(chan struct{})

	controller.Logger.Infof("starting controller")

	controller.informerFactory.Start(controller.stopChan)

	controller.Logger.Infof("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(controller.stopChan, controller.informersSynced...); !ok {
		return fmt.Errorf("could not wait for caches to sync")
	}

	controller.startTime = time.Now().UTC()

	controller.Logger.Infof("starting workers")
	for i := 0; i < nWorkers; i++ {
		n := i
		go func() {
			controller.Logger.Debugf("starting worker #%d", n)
			for !(controller.workQueue.ShuttingDown() && controller.workQueue.Len() == 0) {
				controller.processNextWorkItem()
			}
			controller.Logger.Debugf("stopping worker #%d", n)
		}()
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		controller.syncLogStreams()
	}))

	controller.Logger.Infof("Started controller")

	return nil
}

func (controller *Controller) Stop() error {
	controller.Logger.Infof("Stopping controller")

	if controller.stopChan == nil {
		return fmt.Errorf("controller was not running")
	}

	close(controller.stopChan)
	controller.stopChan = nil

	controller.workQueue.ShutDownWithDrain()

	return nil
}
