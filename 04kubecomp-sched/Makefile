IMAGE = kubecomp-scheduler

.PHONY: build deploy

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o=build/kubecomp-scheduler ./cmd/scheduler

buildImage:
	docker build --no-cache -t ${IMAGE} .

pushImage: 
	kind load docker-image ${IMAGE}

deploy:
	helm install scheduler-plugins charts/ 

remove:
	helm uninstall scheduler-plugins

clean:
	rm -rf bin/
