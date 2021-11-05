FROM golang:1.17-alpine
ARG PORT
WORKDIR /
COPY . .
cmd ["./echo"]
