FROM golang:1.26-alpine

WORKDIR /app

COPY . ./
RUN go mod download

RUN go install github.com/go-delve/delve/cmd/dlv@v1.27.1

RUN go install github.com/air-verse/air@v1.67.4

CMD ["air", "-c", "docker_tests/air.toml"]