package controller

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubedump/pkg/filter"
	"os"
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
	ParentPath string
	Filter     filter.Expression
}

type Controller struct {
	opts          Options
	kubeclientset kubernetes.Interface

	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}

	eventInformerSynced cache.InformerSynced
	podInformerSynced   cache.InformerSynced

	workQueue workqueue.RateLimitingInterface

	// logStreams is a map of a string identifier of a container (<namespace>/<pod>/<container-name>/c<container-id>) and a log stream
	logStreams    map[*os.File]io.ReadCloser
	streamMapLock *sync.RWMutex
}

func NewController(
	kubeclientset kubernetes.Interface,
	opts Options,
) *Controller {
	informerFactory := informers.NewSharedInformerFactory(kubeclientset, time.Second*5)

	eventInformer := informerFactory.Events().V1().Events()
	podInformer := informerFactory.Core().V1().Pods()

	controller := &Controller{
		opts:          opts,
		kubeclientset: kubeclientset,

		informerFactory: informerFactory,
		stopChan:        nil,

		eventInformerSynced: eventInformer.Informer().HasSynced,
		podInformerSynced:   podInformer.Informer().HasSynced,

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		logStreams:    make(map[*os.File]io.ReadCloser),
		streamMapLock: &sync.RWMutex{},
	}

	eventInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.eventHandler,
		UpdateFunc: func(_, obj interface{}) {
			controller.eventHandler(obj)
		},
		DeleteFunc: controller.eventHandler,
	})

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

func (controller *Controller) syncLogStreams(buffer []byte) {
	for file, stream := range controller.logStreams {
		readChan := make(chan int, 1)

		go func() {
			if n, err := stream.Read(buffer); err != nil && err != io.EOF {
				logrus.Errorf("error writing logs to file '%s': %s", file.Name(), err)
			} else {
				readChan <- n
			}
		}()

		select {
		case n := <-readChan:
			if _, err := file.Write(buffer[:n]); err != nil {
				logrus.Errorf("error writing logs to file '%s': %s", file.Name(), err)
			}
		case <-time.After(time.Millisecond):
		}
	}
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

	buffer := make([]byte, 4098)
	go wait.Until(func() {
		controller.syncLogStreams(buffer)
	}, time.Second, controller.stopChan)

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
