# syntax=docker/dockerfile:1

FROM golang:latest AS base

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /lb

FROM gcr.io/distroless/static-debian11

COPY --from=base /lb .

COPY .envdocker /.env

EXPOSE 30000

CMD ["/lb"]