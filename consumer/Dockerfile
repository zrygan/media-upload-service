# build stage
FROM golang:1.27 AS build
WORKDIR /workspace
COPY go.mod go.sum ./
COPY gen/ gen/
COPY util/ util/
COPY consumer/ consumer/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o /out/consumer ./consumer

# run stage
FROM gcr.io/distroless/static-debian12
WORKDIR /workspace
COPY --from=build /out/consumer /consumer
USER nonroot
EXPOSE 9090
ENTRYPOINT ["/consumer"]
