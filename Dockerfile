FROM alpine:3.15.0
ARG PORT
COPY . .
RUN sed -i "s/PORT/$PORT/g" server.json
ENTRYPOINT ["./bp server.json"]
