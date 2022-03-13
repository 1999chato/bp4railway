FROM golang:1.17 as build
COPY . /src
WORKDIR /src

RUN CGO_ENABLED=0 GOOS=linux go build -o bypath .

FROM heroku/heroku:20
ENV HOME /app
WORKDIR /app
COPY --from=build /src/bypath /app
CMD ["./bypath"]
