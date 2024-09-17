package main

import (
	"log"
	"my-scheduler-plugins/pkg/falconresources"
	"os"

	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
	log.Printf("falconresources-scheduler starts!\n")
	command := app.NewSchedulerCommand(
		app.WithPlugin(falconresources.Name, falconresources.New),
	)

	code := cli.Run(command)
	os.Exit(code)
}
