FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mockmcp ./cmd/mockmcp

FROM alpine:3.21 AS mockmcp

RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/mockmcp /usr/local/bin/mockmcp
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/mockmcp"]

FROM alpine:3.21 AS agent

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/server /usr/local/bin/server
COPY configs /app/configs

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
CMD ["-config", "configs/agent.yaml"]
