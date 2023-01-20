package main

import (
	"fmt"
	kubedumpcmd "github.com/joshmeranda/kubedump/pkg/cmd"
	"os"
)

func main() {
	app := kubedumpcmd.NewKubedumpServerApp()

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
