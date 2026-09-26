FROM golang:1.26-alpine AS air
RUN --mount=type=cache,id=sensor-hub-dev-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=sensor-hub-dev-go-build,target=/root/.cache/go-build \
    go install github.com/air-verse/air@v1.67.4

FROM golang:1.26-alpine AS dlv
RUN --mount=type=cache,id=sensor-hub-dev-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=sensor-hub-dev-go-build,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.27.1

FROM golang:1.26-alpine
WORKDIR /app
COPY --from=air /go/bin/air /go/bin/air
COPY --from=dlv /go/bin/dlv /go/bin/dlv
COPY go.mod go.sum ./
RUN go mod download
COPY . .
CMD ["air", "-c", "devstack/air.toml"]
