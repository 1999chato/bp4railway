FROM golang:1.16-alpine
ARG PORT
WORKDIR /
COPY . .
RUN go mod download
RUN go build -o /echo-server ./echo-server
ENTRYPOINT ["./echo-server"]
