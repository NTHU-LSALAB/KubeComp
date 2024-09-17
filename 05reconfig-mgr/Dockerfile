FROM golang:1.20

WORKDIR /
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/reconfig-mgr cmd/app/main.go

FROM alpine:latest

COPY --from=0 build/reconfig-mgr /bin/reconfig-mgr
