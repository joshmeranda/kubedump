package controller

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	informersappsv1 "k8s.io/client-go/informers/apps/v1"
	informersbatchv1 "k8s.io/client-go/informers/batch/v1"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubedump/pkg/filter"
	"sync"
	"time"
)

type Options struct {
	ParentPath string
	Filter     filter.Expression
}

type Controller struct {
	Options

	kubeclientset kubernetes.Interface
	startTime     time.Time

	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}

	sieve Sieve

	// logStreams is a store of logStreams mappe to a unique identifier for the associated container.
	logStreams   map[string]Stream
	logStreamsMu sync.Mutex

	workQueue workqueue.RateLimitingInterface

	informersSynced []cache.InformerSynced

	podInformer        informerscorev1.PodInformer
	serviceInformer    informerscorev1.ServiceInformer
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

	controller := &Controller{
		Options:       opts,
		kubeclientset: kubeclientset,

		informerFactory: informerFactory,
		stopChan:        nil,

		sieve: sieve,

		logStreams: make(map[string]Stream),

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		podInformer:        informerFactory.Core().V1().Pods(),
		serviceInformer:    informerFactory.Core().V1().Services(),
		jobInformer:        informerFactory.Batch().V1().Jobs(),
		replicasetInformer: informerFactory.Apps().V1().ReplicaSets(),
		deploymentInformer: informerFactory.Apps().V1().Deployments(),
	}

	eventInformer := informerFactory.Events().V1().Events()

	controller.informersSynced = []cache.InformerSynced{
		eventInformer.Informer().HasSynced,

		controller.podInformer.Informer().HasSynced,
		controller.serviceInformer.Informer().HasSynced,
		controller.jobInformer.Informer().HasSynced,
		controller.replicasetInformer.Informer().HasSynced,
		controller.deploymentInformer.Informer().HasSynced,
	}

	handler := cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			return sieve.Matches(obj)
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.onAdd,
			UpdateFunc: controller.onUpdate,
			DeleteFunc: controller.onDelete,
		},
	}

	eventInformer.Informer().AddEventHandler(handler)

	controller.podInformer.Informer().AddEventHandler(handler)
	controller.serviceInformer.Informer().AddEventHandler(handler)

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

	for id, stream := range controller.logStreams {
		if err := stream.Sync(); err != nil {
			logrus.Errorf("error syncing container '%s'", id)
		} else {
			logrus.Debugf("synced logs for containr '%s'", id)
		}
	}

	controller.logStreamsMu.Unlock()

	time.Sleep(time.Second)

	controller.workQueue.AddRateLimited(NewJob(func() {
		controller.syncLogStreams()
	}))
}

func (controller *Controller) processNextWorkItem() bool {
	obj, shutdown := controller.workQueue.Get()

	if shutdown {
		return false
	}

	job, ok := obj.(Job)

	if !ok {
		logrus.Errorf("could not understand worker function")
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

	logrus.Infof("starting controller")

	controller.informerFactory.Start(controller.stopChan)

	logrus.Infof("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(controller.stopChan, controller.informersSynced...); !ok {
		return fmt.Errorf("could not wait for caches to sync")
	}

	controller.startTime = time.Now().UTC()

	logrus.Infof("starting workers")
	for i := 0; i < nWorkers; i++ {
		n := i
		go func() {
			logrus.Debugf("starting worker #%d", n)
			for controller.processNextWorkItem() {
				/* do nothing */
			}
		}()
	}

	controller.workQueue.AddRateLimited(NewJob(func() {
		controller.syncLogStreams()
	}))

	logrus.Infof("Started controller")

	return nil
}

func (controller *Controller) Stop() error {
	if controller.stopChan == nil {
		return fmt.Errorf("controller was not running")
	}

	logrus.Infof("Stopping controller")

	close(controller.stopChan)
	controller.stopChan = nil
	controller.workQueue.ShutDown()

	return nil
}
