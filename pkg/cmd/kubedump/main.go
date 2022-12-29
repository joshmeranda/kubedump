package main

import (
	"fmt"
	kubedumpcmd "kubedump/pkg/cmd"
	"os"
	"os/signal"
)

func main() {
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt)

	stopChan := make(chan interface{})
	go func() {
		<-signalChan
		fmt.Printf("recevied interrupt, stopping kubedumpcmd")
		close(stopChan)
	}()

	app := kubedumpcmd.NewKubedumpApp(stopChan)

	if err := app.Run(os.Args); err != nil {
		kubedumpcmd.CmdLogger.Infof("%s", err)
		os.Exit(1)
	}
}
