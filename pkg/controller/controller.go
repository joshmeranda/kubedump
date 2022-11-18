package controller

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"kubedump/pkg/filter"
	"time"
)

//go:generate go run ../codegen -p apiappsv1.ReplicaSet -i "apiappsv1 \"k8s.io/api/apps/v1\""
//go:generate go run ../codegen -p apiappsv1.Deployment -i "apiappsv1 \"k8s.io/api/apps/v1\""
//go:generate go run ../codegen -p apibatchv1.Job -i "apibatchv1 \"k8s.io/api/batch/v1\""
//go:generate go run ../codegen -p apicorev1.Service -c apimetav1.Condition -i "apicorev1 \"k8s.io/api/core/v1\",apimetav1 \"k8s.io/apimachinery/pkg/apis/meta/v1\""
//go:generate go fmt deployment.go job.go replicaset.go service.go

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
	StartTime  time.Time
}

type Controller struct {
	opts          Options
	kubeclientset kubernetes.Interface

	informerFactory informers.SharedInformerFactory
	stopChan        chan struct{}

	informersSynced []cache.InformerSynced

	workQueue workqueue.RateLimitingInterface
}

func NewController(
	kubeclientset kubernetes.Interface,
	opts Options,
) *Controller {
	informerFactory := informers.NewSharedInformerFactory(kubeclientset, time.Second*5)

	eventInformer := informerFactory.Events().V1().Events()
	podInformer := informerFactory.Core().V1().Pods()
	serviceInformer := informerFactory.Core().V1().Services()
	jobInformer := informerFactory.Batch().V1().Jobs()
	replicasetInformer := informerFactory.Apps().V1().ReplicaSets()
	deploymentInformer := informerFactory.Apps().V1().Deployments()

	informersSynced := []cache.InformerSynced{
		eventInformer.Informer().HasSynced,
		podInformer.Informer().HasSynced,
		serviceInformer.Informer().HasSynced,
		jobInformer.Informer().HasSynced,
		replicasetInformer.Informer().HasSynced,
		deploymentInformer.Informer().HasSynced,
	}

	controller := &Controller{
		opts:          opts,
		kubeclientset: kubeclientset,

		informerFactory: informerFactory,
		stopChan:        nil,

		informersSynced: informersSynced,

		workQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	eventInformer.Informer().AddEventHandler(NewEventHandler(opts, controller.workQueue, podInformer, jobInformer))

	podInformer.Informer().AddEventHandler(NewPodHandler(opts, controller.workQueue, kubeclientset))
	serviceInformer.Informer().AddEventHandler(NewServiceHandler(opts, controller.workQueue))

	jobInformer.Informer().AddEventHandler(NewJobHandler(opts, controller.workQueue))

	replicasetInformer.Informer().AddEventHandler(NewReplicaSetHandler(opts, controller.workQueue))
	deploymentInformer.Informer().AddEventHandler(NewDeploymentHandler(opts, controller.workQueue))

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
	if ok := cache.WaitForCacheSync(controller.stopChan, controller.informersSynced...); !ok {
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
