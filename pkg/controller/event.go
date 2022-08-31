package controller

import (
	"github.com/sirupsen/logrus"
	eventsv1 "k8s.io/api/events/v1"
)

type EventHandler struct{}

func (handler *EventHandler) onEvent(obj interface{}) {
	event, ok := obj.(*eventsv1.Event)

	if !ok {
		logrus.Errorf("could not coerce object to event")
	}

	logrus.Infof("Received event: %s [%s] %s", event.Regarding.Kind, event.EventTime.String(), event.Note)
}

func (handler *EventHandler) OnAdd(obj interface{}) {
	handler.onEvent(obj)
}

func (handler *EventHandler) OnUpdate(_ interface{}, obj interface{}) {
	handler.onEvent(obj)
}

func (handler *EventHandler) OnDelete(obj interface{}) {
	handler.onEvent(obj)
}
