FROM golang:1.25-alpine

WORKDIR /app

RUN apk add --no-cache docker-cli

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o failsafe ./cmd/controller/main.go

EXPOSE 8000

CMD ["./failsafe"]