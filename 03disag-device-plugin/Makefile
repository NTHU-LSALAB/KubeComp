IMAGE = disag-device-plugin

.PHONY: build deploy

build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/falcon cmd/server/app.go

buildImage:
	docker build --no-cache -t ${IMAGE} .

pushImage:
	kind load docker-image ${IMAGE}

deploy:
	helm install falcon charts/
	
remove:
	helm uninstall falcon
