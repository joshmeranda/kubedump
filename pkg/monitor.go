package kdump

import (
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type Monitor interface{}

type PodMonitor struct {
	watcher watch.Interface
}

func MonitorPods(watcher watch.Interface) (PodMonitor, error) {
	monitor := PodMonitor{watcher: watcher}

	c := monitor.watcher.ResultChan()

	go func() {
		for {
			switch event := <-c; event.Type {
			case watch.Added, watch.Modified:
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					logrus.Errorf("error getting pod data from event")
				}

				_ = pod
			case watch.Deleted:
			case watch.Error:
			}
		}
	}()

	logrus.Info("monitoring")
	return monitor, nil
}

func (pm *PodMonitor) Stop() {
	pm.watcher.Stop()
}
