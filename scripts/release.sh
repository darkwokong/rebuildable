#!/usr/bin/env sh
set -eu

: "${VERSION:?VERSION is required, for example VERSION=0.1.0 make release}"

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist="$root/dist/$VERSION"
mkdir -p "$dist"

for target in \
  "linux amd64" \
  "linux arm64" \
  "darwin amd64" \
  "darwin arm64" \
  "windows amd64"; do
  set -- $target
  goos=$1
  goarch=$2
  suffix=""
  case "$goos" in
    windows) suffix=".exe" ;;
  esac
  name="rebuildable_${VERSION}_${goos}_${goarch}"
  output="$dist/$name$suffix"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -mod=vendor -trimpath \
      -ldflags "-s -w -X github.com/jayeshsarvaiya/rebuildable/internal/version.Version=$VERSION" \
      -o "$output" ./cmd/rebuildable
  tar -C "$dist" -czf "$dist/$name.tar.gz" "$(basename "$output")"
  rm -f "$output"
done

(cd "$dist" && sha256sum *.tar.gz > SHA256SUMS)
printf 'release artifacts: %s\n' "$dist"
