FROM golang:alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY *.go /app
RUN go build .

CMD ["./manyporter"]