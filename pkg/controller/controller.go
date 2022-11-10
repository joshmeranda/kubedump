package controller

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	informerscorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubedump/pkg/filter"
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
	ParentPath string
	Filter     filter.Expression
}

type Controller struct {
	opts          Options
	kubeclientset kubernetes.Interface

	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}

	podHandler *PodHandler

	eventInformerSynced      cache.InformerSynced
	podInformerSynced        cache.InformerSynced
	jobInformerSynced        cache.InformerSynced
	replicasetInformerSynced cache.InformerSynced
	deploymentInformerSynced cache.InformerSynced

	podInformer informerscorev1.PodInformer

	workQueue workqueue.RateLimitingInterface
}

func NewController(
	kubeclientset kubernetes.Interface,
	opts Options,
) *Controller {
	informerFactory := informers.NewSharedInformerFactory(kubeclientset, time.Second*5)

	eventInformer := informerFactory.Events().V1().Events()
	podInformer := informerFactory.Core().V1().Pods()
	jobInformer := informerFactory.Batch().V1().Jobs()
	replicasetInformer := informerFactory.Apps().V1().ReplicaSets()
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	controller := &Controller{
		opts:          opts,
		kubeclientset: kubeclientset,

		informerFactory: informerFactory,
		stopChan:        nil,

		eventInformerSynced:      eventInformer.Informer().HasSynced,
		podInformerSynced:        podInformer.Informer().HasSynced,
		jobInformerSynced:        jobInformer.Informer().HasSynced,
		replicasetInformerSynced: replicasetInformer.Informer().HasSynced,
		deploymentInformerSynced: deploymentInformer.Informer().HasSynced,

		podInformer: podInformer,

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	eventInformer.Informer().AddEventHandler(NewEventHandler(controller.opts, controller.workQueue, podInformer, jobInformer))

	controller.podHandler = NewPodHandler(controller.opts, controller.workQueue, controller.kubeclientset)
	podInformer.Informer().AddEventHandler(controller.podHandler)

	jobInformer.Informer().AddEventHandler(NewJobHandler(controller.opts, controller.workQueue))

	replicasetInformer.Informer().AddEventHandler(NewReplicasetHandler(opts, controller.workQueue))
	deploymentInformer.Informer().AddEventHandler(NewDeploymentHandler(controller.opts, controller.workQueue))

	return controller
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
	if ok := cache.WaitForCacheSync(controller.stopChan, controller.eventInformerSynced, controller.podInformerSynced); !ok {
		return fmt.Errorf("could not wait for caches to sync")
	}

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

	controller.workQueue.AddRateLimited(NewJob(controller.podHandler.syncLogStreams))

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
