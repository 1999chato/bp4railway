FROM alpine:3.15.0
ARG PORT=8080
WORKDIR /
COPY . .
RUN echo $PORT
RUN sed -i "s/PORT/${PORT}/g" config.json
RUN cat config.json
CMD ./bp
