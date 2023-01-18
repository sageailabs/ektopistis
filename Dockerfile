FROM golang:1.18 as build

WORKDIR /go/src/app
COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /go/bin/ektopistis

FROM gcr.io/distroless/static-debian11

COPY --from=build /go/bin/ektopistis /ektopistis
ENTRYPOINT ["/ektopistis"]
