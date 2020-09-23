FROM golang:1.14.6 as builder
ARG BUILD="n/a"
WORKDIR /src/
COPY * ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /serve -ldflags="-X main.Build=${BUILD}" ./serve.go

FROM scratch
COPY --from=builder /serve /serve

ENTRYPOINT ["/serve", "--message='hello from {{addr}}'"]
