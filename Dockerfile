FROM golang:1.16-alpine
ARG PORT
WORKDIR /
COPY *.go ./

CMD ["go","run","main.go"]