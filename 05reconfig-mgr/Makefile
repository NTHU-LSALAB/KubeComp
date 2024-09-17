IMAGE = reconfig-daemon

.PHONY: build

build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/falcon cmd/app/main.go

buildImage:
	docker build -t ${IMAGE} . --no-cache

pushImage:
	kind load docker-image ${IMAGE}

deploy:
	helm install reconfig-daemon chart/
	
remove:
	helm uninstall reconfig-daemon
