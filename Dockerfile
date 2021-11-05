FROM golang:1.17-alpine
ARG PORT
WORKDIR /go/src/echo-server
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./echo ./echo-server
ENTRYPOINT ["./echo"]
