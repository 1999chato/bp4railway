FROM alpine:3.15.0
ARG PORT
WORKDIR /
COPY . .
RUN echo $PORT
RUN sed -i "s/PORT/${PORT}/g" server.json
CMD ./bp server.json
