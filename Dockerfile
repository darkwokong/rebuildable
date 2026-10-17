FROM golang:1.27.2-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN git init -q \
    && git config user.email rebuildable-image@example.invalid \
    && git config user.name Rebuildable-Image \
    && git add . \
    && git commit -q -m "image snapshot" \
    && git commit --allow-empty -q -m "image history" \
    && git commit --allow-empty -q -m "image history 2"
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags=-s -o /out/rebuildable ./cmd/rebuildable

FROM build AS rebuildable-test

CMD ["go", "test", "./..."]

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
    && apt-get install --no-install-recommends --yes git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/rebuildable /usr/local/bin/rebuildable
COPY --from=build /src /app

ENTRYPOINT ["rebuildable"]
CMD ["audit", "/app"]
