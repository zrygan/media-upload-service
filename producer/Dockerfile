FROM golang:1.27

WORKDIR /workspace

COPY . .

RUN go build -o run ./producer

CMD ["./run"]