FROM golang:1.14.6 as builder
WORKDIR /src/
COPY * ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /request ./request.go

FROM scratch
COPY --from=builder /request /request

ENTRYPOINT ["/request"]
