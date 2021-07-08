FROM golang:1.16-alpine AS build

WORKDIR /app
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o goas ./cmd/goas/main.go

FROM alpine:3.10
COPY --from=build /app/goas /goas
CMD ["/goas", "-d", "/data", "-s", "/resources"]
