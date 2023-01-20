package main

import (
	"fmt"
	kubedumpcmd "github.com/joshmeranda/kubedump/pkg/cmd"
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
		fmt.Println(err)
		os.Exit(1)
	}
}
