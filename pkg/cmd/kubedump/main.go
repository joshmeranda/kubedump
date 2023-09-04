package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	kubedumpcmd "github.com/joshmeranda/kubedump/pkg/cmd"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-signalChan
		fmt.Printf("recevied interrupt, stopping kubedumpcmd\n")
		cancel()
	}()

	app := kubedumpcmd.NewKubedumpApp()

	// if err := app.RunContext(ctx, os.Args); err != nil {
	if err := app.RunContext(ctx, []string{"kubedump", "dump", "--filter", "namespace default"}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
