FROM golang:1.20

WORKDIR /app

# copy files
COPY . .

# install modules
RUN go mod download

# build
RUN CGO_ENABLED=0 GOOS=linux go build -o monday

# EXPOSE 8081

# run
CMD ["./monday"]