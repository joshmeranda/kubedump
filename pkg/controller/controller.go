package controller

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubedump/pkg/filter"
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

	podInformerSynced cache.InformerSynced

	workQueue workqueue.RateLimitingInterface
}

func NewController(
	kubeclientset kubernetes.Interface,
	podInformer coreinformers.PodInformer,
	opts Options,
) *Controller {
	controller := &Controller{
		opts:              opts,
		kubeclientset:     kubeclientset,
		podInformerSynced: podInformer.Informer().HasSynced,
		workQueue:         workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.podAddHandler,
		UpdateFunc: controller.podUpdateHandler,
		DeleteFunc: controller.podDeletedHandler,
	})

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

func (controller *Controller) Run(nWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	logrus.Infof("starting controller")

	logrus.Infof("waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, controller.podInformerSynced); !ok {
		return fmt.Errorf("could not wait for caches to sync")
	}

	logrus.Infof("starting workers")
	for i := 0; i < nWorkers; i++ {
		n := i
		go func() {
			logrus.Debugf("starting worker #%d", n)
			for controller.processNextWorkItem() { /* do nothing */
			}
		}()
	}

	logrus.Infof("Started controller")
	<-stopCh
	logrus.Infof("Stopping controller")

	return nil
}
