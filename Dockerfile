FROM golang:1.20-alpine

WORKDIR /usr/src/app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o monday

EXPOSE 8082

CMD ["./monday"]