package main

import (
	"my-device-plugin/pkg/server"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
)

func main() {
	log.Info("Disaggregated device plugin starts.")
	diagDevSrv := server.NewDisagDevServer()
	go diagDevSrv.Run()

	// Registers with Kubelet
	if err := diagDevSrv.RegisterToKubelet(); err != nil {
		log.Fatalf("Failed to register with Kubelet: %v", err)
	}
	log.Info("Successfully registered with Kubelet.")

	// Listens to kubelet.sock
	devicePluginSocket := filepath.Join(server.DevicePluginPath, server.KubeletSocket)
	log.Infof("Device plugin socket: %s", devicePluginSocket)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create FS watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(server.DevicePluginPath); err != nil {
		log.Fatalf("Failed to watch path %s: %v", server.DevicePluginPath, err)
	}
	log.Info("Watching for changes on kubelet.sock")

	for {
		select {
		case event := <-watcher.Events:
			if event.Name == devicePluginSocket && event.Op&fsnotify.Create == fsnotify.Create {
				time.Sleep(time.Second)
				log.Fatalf("inotify: %s created, restarting.", devicePluginSocket)
			}
		case err := <-watcher.Errors:
			log.Fatalf("inotify: %s", err)
		}
	}
}
