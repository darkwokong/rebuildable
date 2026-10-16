FROM golang:1.27.2-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags=-s -o /out/rebuildable ./cmd/rebuildable

FROM build AS rebuildable-test

CMD ["go", "test", "./..."]

FROM debian:bookworm-slim AS runtime

WORKDIR /app
COPY --from=build /out/rebuildable /usr/local/bin/rebuildable
COPY . /app

ENTRYPOINT ["rebuildable"]
CMD ["audit", "/app"]
