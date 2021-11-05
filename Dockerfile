FROM golang:1.17-alpine
ARG PORT
WORKDIR /
COPY . .
ENTRYPOINT ["./echo"]
