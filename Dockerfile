FROM golang:1.25-alpine




WORKDIR /app
# Install all required dependencies for Go, Docker, Java, unzip, and aapt on Alpine
RUN apk add --no-cache \
    docker-cli \
    wget \
    unzip \
    openjdk17-jre \
    ca-certificates \
    bash



# Copy provided aapt binary and make it executable
COPY tools/aapt /usr/local/bin/aapt
RUN chmod +x /usr/local/bin/aapt


COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN go mod download


COPY . .

RUN go build -o failsafe ./cmd/controller/main.go

EXPOSE 8000

CMD ["./failsafe"]