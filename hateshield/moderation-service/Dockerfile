FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN go build -o /service ./cmd
FROM gcr.io/distroless/base-debian12
COPY --from=build /service /service
ENTRYPOINT ["/service"]
