FROM golang:1.27.2-bookworm AS build
WORKDIR /src
COPY . .
RUN go build -o /out/good-go .

FROM debian:bookworm-slim
COPY --from=build /out/good-go /usr/local/bin/good-go
ENTRYPOINT ["good-go"]
