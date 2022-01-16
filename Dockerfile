FROM alpine:3.15.0
WORKDIR /
COPY . .
CMD ./start.sh
