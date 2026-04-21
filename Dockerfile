# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates git
ENV PATH="/usr/local/go/bin:${PATH}"

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/huectl ./cmd/huectl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/huectl /usr/local/bin/huectl
ENTRYPOINT ["/usr/local/bin/huectl"]
