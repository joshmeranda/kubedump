package main

import (
	"fmt"
	kubedump "kubedump/pkg/cmd"
	"os"
)

func main() {
	// todo: handle ctrl-c
	app := kubedump.NewKubedumpApp(nil)

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
}
