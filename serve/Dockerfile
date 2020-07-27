FROM golang:1.14.6 as builder
WORKDIR /src/
COPY * ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /serve ./serve.go

FROM scratch
COPY --from=builder /serve /serve

ENTRYPOINT ["/serve", "--address=:8080", "--message='hello from {{addr}}'"]
