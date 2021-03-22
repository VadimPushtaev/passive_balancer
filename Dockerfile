FROM golang:1.15

WORKDIR /app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["passive_balancer"]