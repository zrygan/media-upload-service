# media-upload-service

> Also see: <https://github.com/zrygan/fault-tolerant-ems>

## Dependencies

Go, FFmpeg

Without Docker, `ffmpeg` must be on your `PATH` for both programs. The producer uses it for duplicate detection, and the consumer's workers use it to compress each video (H.264, at most 720p) before saving it. If compression fails or doesn't make the file smaller, the consumer saves the original. Both Docker images install FFmpeg for you.

To run with Docker you only need [Docker Desktop](https://www.docker.com/products/docker-desktop/) (or Docker Engine with the Compose plugin). Go is installed inside the images.

## Project layout

| Path | What it is |
| --- | --- |
| `producer/` | Producer program and its `Dockerfile` |
| `consumer/` | Consumer program and its `Dockerfile` |
| `configs/` | JSON config files, mounted into the containers |
| `videos/` | Videos to upload, one subfolder per producer thread (not committed) |
| `data/media/` | Where the consumer saves uploaded videos (not committed) |
| `data/staging/` | Where the consumer keeps uploads that are still arriving, before moving them to `data/media/` (not committed) |
| `docker-compose.yaml` | Builds and runs the producer and consumer together |

## Build without Docker

From the repo root:

```sh
go build -o producer-bin ./producer
go build -o consumer-bin ./consumer
```

The producer reads its config from `.configs/producer.json` by default. Use `-config <path>` to point it somewhere else:

```sh
./producer-bin -config configs/producer.json
```

## Run with Docker Compose

### 1. Create the video folders

Make one folder per producer thread under `videos/` and put some videos in each:

```text
videos/
├── up1/
├── up2/
├── up3/
└── up4/
```

Also create the consumer's folders:

```text
data/
├── media/
└── staging/
```

The consumer checks that both exist when it starts. Docker creates `data/` if it is missing, but not the folders inside it. `videos/` and `data/` are in `.gitignore` and `.dockerignore`.

`staging/` and `media/` must stay inside the same `data/` folder. Compose mounts `data/` as a single volume, so moving a finished upload from `staging/` to `media/` is a rename on one filesystem. If they were separate mounts, the rename would fail.

### 2. Check the producer config

[configs/producer.json](configs/producer.json):

```json
{
    "thread_count": 4,
    "folder_list": ["/videos/up1", "/videos/up2", "/videos/up3", "/videos/up4"],
    "consumer_address": "172.28.0.10:9090"
}
```

| Field | Meaning |
| --- | --- |
| `thread_count` | Number of producer threads (`p`). Must be positive. |
| `folder_list` | One folder per thread. The count must equal `thread_count`. These are paths **inside the container**: `./videos` on your machine is mounted at `/videos`. |
| `consumer_address` | `host:port` of the consumer's gRPC server. `172.28.0.10` is the consumer's fixed address on the Compose network. |

To change `p`, edit `thread_count`, add or remove entries in `folder_list`, and create the matching folders under `videos/`.

### 3. Build and start

From the repo root:

```sh
docker compose up --build
```

Compose builds both images, creates the `backend` network (`172.28.0.0/24`), and starts the consumer first, then the producer.

To stop everything, press `Ctrl+C`, then run:

```sh
docker compose down
```

### What Compose sets up

| Service | Address on `backend` | Ports published on the host | Mounts |
| --- | --- | --- | --- |
| `consumer` | `172.28.0.10` | `9090` → `9090` (gRPC) | `./configs` → `/workspace/.configs` (read-only), `./data` → `/workspace/data` |
| `producer` | `172.28.0.11` | none | `./configs` → `/workspace/.configs` (read-only), `./videos` → `/videos` (read-only) |

Check the Compose file without building anything:

```sh
docker compose config
```

## Run on two machines (separate VMs)

The producer and consumer are built and run independently, so each can go on its own VM. Install Docker and copy the repo to both.

**Consumer VM:**

```sh
docker compose up --build consumer
```

**Producer VM:**

1. In `configs/producer.json`, set `consumer_address` to the consumer VM's IP and the published port, for example `"192.168.1.20:9090"`. The `172.28.0.x` addresses only exist inside Docker on one machine, so they can't be used across VMs.
2. Start only the producer. `--no-deps` stops Compose from also starting a consumer on this VM:

   ```sh
   docker compose up --build --no-deps producer
   ```

Make sure the consumer VM's firewall allows incoming connections on port `9090`.
