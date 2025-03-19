FROM golang:1.24 as build

WORKDIR /go/src/app
COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /go/bin/ektopistis

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /go/bin/ektopistis /ektopistis
ENTRYPOINT ["/ektopistis"]
