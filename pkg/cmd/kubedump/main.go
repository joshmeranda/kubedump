package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	kubedump "kubedump/pkg/cmd"
	"os"
	"os/signal"
)

func main() {
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt)

	stopChan := make(chan interface{})
	go func() {
		<-signalChan
		logrus.Infof("recevied interrupt, stopping kubedump")
		close(stopChan)
	}()

	app := kubedump.NewKubedumpApp(stopChan)

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
}
