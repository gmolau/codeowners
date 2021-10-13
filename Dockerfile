# Dockerfile for use as GitHub Action

# Builder
FROM golang:1.16.7-alpine3.13 as builder

WORKDIR /build
COPY main.go lib.go go.mod go.sum ./

RUN GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -a -v -o codeowners .

# Runner
FROM busybox:1.33.1

COPY --from=builder /build/codeowners /codeowners
COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
