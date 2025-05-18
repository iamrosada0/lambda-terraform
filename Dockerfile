FROM golang:1.24.3

RUN apt-get update && apt-get install -y zip

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .