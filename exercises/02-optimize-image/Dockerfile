FROM golang:1.22

WORKDIR /app

COPY . .

RUN go build -o quote-engine .

EXPOSE 8080

CMD ["./quote-engine"]
