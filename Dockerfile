FROM golang:1.17-alpine
ARG PORT
WORKDIR /
COPY . .
export CGO_ENABLED=0
cmd ["sh","./echo"]
