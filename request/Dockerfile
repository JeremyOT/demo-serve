FROM golang:1.14.6 as builder
ARG BUILD="n/a"
WORKDIR /src/
COPY * ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /request -ldflags="-X main.Build=${BUILD}" ./request.go

FROM scratch
COPY --from=builder /request /request

ENTRYPOINT ["/request"]
