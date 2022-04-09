FROM golang:1.18-alpine AS build

WORKDIR /app
COPY ./go.* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o goas -ldflags="-s -w" ./cmd/goas/main.go

FROM alpine:3.10
COPY --from=build /app/goas /goas
CMD ["/goas", "--data", "/data", "--samples", "/resources/samples", "--dictionary", "/resources/banlist.txt"]
