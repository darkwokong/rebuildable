FROM golang:1.27.2-bookworm AS rebuildable-test

WORKDIR /app
COPY . .

FROM rebuildable-test AS runtime
CMD ["go", "run", "."]
