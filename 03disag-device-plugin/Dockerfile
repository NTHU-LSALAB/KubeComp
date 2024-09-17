FROM golang:1.20

WORKDIR /
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/falcon cmd/server/app.go

FROM alpine:latest

COPY --from=0 build/falcon /bin/falcon

CMD ["/bin/falcon"]
