FROM golang:alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY *.go /app
RUN go build -o manyporter .

FROM scratch AS final
WORKDIR /app
COPY --from=build /app/manyporter manyporter
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
CMD ["./manyporter"]