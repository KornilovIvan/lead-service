FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BINARY=./cmd/app
RUN CGO_ENABLED=0 go build -o /service ${BINARY}

FROM alpine:3.19
COPY --from=build /service /service
ENTRYPOINT ["/service"]
