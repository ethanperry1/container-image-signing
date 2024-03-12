###############
# Build image #
###############
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21-cbl-mariner2.0 AS builder

COPY ./cmd/trusted /app/cmd
COPY ./go.mod /app/go.mod

WORKDIR /app

RUN go build -o ./bin/app ./cmd

CMD ["/app/bin/app"]