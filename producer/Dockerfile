FROM golang:1.27

RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY . .

RUN go build -o run ./producer

CMD ["./run"]