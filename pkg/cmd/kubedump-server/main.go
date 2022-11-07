package main

import (
	"github.com/sirupsen/logrus"
	kubedump "kubedump/pkg/cmd"
	"os"
)

func main() {
	app := kubedump.NewKubedumpServerApp()

	if err := app.Run(os.Args); err != nil {
		logrus.Errorf("Error: %s", err)
	}
}
