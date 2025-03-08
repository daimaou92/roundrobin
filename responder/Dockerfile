# syntax=docker/dockerfile:1

FROM golang:latest AS base

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /responder

FROM gcr.io/distroless/static-debian11

COPY --from=base /responder .

EXPOSE 20000

CMD ["/responder"]