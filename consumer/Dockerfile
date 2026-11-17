# build stage
FROM golang:1.27 AS build
WORKDIR /workspace
COPY go.mod go.sum ./
COPY gen/ gen/
COPY util/ util/
COPY consumer/ consumer/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/consumer ./consumer \
 && CGO_ENABLED=0 go build -o /out/webapp ./consumer/webapp

# run stage (the GUI runs from the same image, see docker-compose.yaml)
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /workspace
COPY --from=build /out/consumer /consumer
COPY --from=build /out/webapp /webapp
USER 65532:65532
EXPOSE 9090 8008
ENTRYPOINT ["/consumer"]
