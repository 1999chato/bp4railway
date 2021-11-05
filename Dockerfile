FROM golang:1.17-alpine
ARG PORT
WORKDIR /
COPY . .
RUN export CGO_ENABLED=0
CMD ["sh","./echo"]
