FROM golang:1.20

WORKDIR /
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o=resource-pool ./resource-pool.go

FROM alpine

COPY --from=0 resource-pool /bin/resource-pool

