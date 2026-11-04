// Scope for now: the design decisions and the producer.
//
// Slides the consumer still has to present are #consumer(...) placeholders,
// search for "#consumer(" to list them.

#import "@preview/mmdr:0.2.2": mermaid-svg

#set page(
  paper: "presentation-16-9",
  margin: (x: 2cm, y: 1.5cm),
  header: none,
  footer: none,
)

#set text(size: 20pt, font: "Inter")

#let highlighted-code(lines: (), size: 11pt, body) = [
  #show raw.where(block: true): block.with(
    fill: rgb("#f8f9fa"),
    inset: (x: 12pt, y: 8pt),
    radius: 6pt,
    stroke: 0.5pt + rgb("#e2e8f0"),
  )
  #show raw: set text(size: size, font: "Cousine")
  #show raw.line: it => {
    let is-target = it.number in lines
    let row = box(
      width: 100%,
      fill: if is-target { rgb("#fde8e8") } else { none },
      outset: (x: 4pt, y: 1.2pt),
      radius: 3pt,
      it,
    )
    if is-target or lines.len() == 0 { row } else { text(fill: rgb("#8a939e"), row) }
  }
  #body
]

#let table-stroke = 0.5pt + rgb("#e2e8f0")
#let table-fill = (x, y) => if y == 0 { rgb("#f1f5f9") } else if calc.even(y) { rgb("#f8fafc") } else { none }

// Renders Mermaid source, in the deck's font and colors.
#let mermaid(code, width: 100%) = align(center, image(
  bytes(mermaid-svg(code, theme: (
    font_family: "Inter",
    font_size: 13,
    primary_color: "#f8f9fa",
    primary_border_color: "#94a3b8",
    primary_text_color: "#111827",
    line_color: "#64748b",
  ))),
  format: "svg",
  width: width,
))

// Placeholder for a slide the consumer still has to present.
#let consumer(topic, body) = align(center + horizon)[
  #text(size: 40pt, weight: "bold", fill: rgb("#b91c1c"))[CONSUMER]

  #v(4pt)
  #text(size: 20pt, weight: "semibold")[#topic]

  #v(8pt)
  #align(left)[
    #set text(size: 15pt, fill: rgb("#6b7280"))
    #body
  ]
]

// Title Slide
#align(center + horizon)[
  #text(size: 40pt, weight: "bold")[Media Upload Service]

  #v(8pt)
  #text(size: 20pt, fill: rgb("#6b7280"))[Enrique Aragon, Stephen Borja, Erin Chua, & Zhean Robby Ganituen]
]

#pagebreak()

#align(center + horizon)[
  = The Problem
]

#pagebreak()
== Media Upload Service
A two-machine simulation of a video upload service.

- *Producer (video uploader):* $p$ threads, each representing a user. Each thread is assigned a folder and uploads every video in it to the consumer.
- *Consumer (video server):* accepts many uploads at once and holds them in a queue of size $q$. Then, $c$ workers save them.
  - *Leaky bucket:* if the queue is full, a new upload is *dropped*, not blocked.

#pagebreak()
== Architecture
#mermaid("flowchart LR
  subgraph P[Producer]
    F1[Folder 1] --> G1[Goroutine 1]
    F2[Folder 2] --> G2[Goroutine 2]
    F3[Folder p] --> G3[Goroutine p]
  end
  subgraph C[Consumer]
    A[Admission check] --> Q[Queue of size q]
    Q --> W1[Worker 1]
    Q --> W2[Worker 2]
    Q --> W3[Worker c]
  end
  G1 -->|gRPC| A
  G2 -->|gRPC| A
  G3 -->|gRPC| A
")

- These slides cover the *producer* and the decisions that affect both programs.
- The two programs share no memory. They coordinate only through gRPC messages, defined in the `.proto` file.

#pagebreak()
== Producer Input & Output
#grid(
  columns: (1fr, 1fr),
  gutter: 24pt,
  [
    *Input* (`-config` JSON file):
    #text(size: 13pt)[
      ```json
      {
        "thread_count": 2,
        "folder_list": ["videos/a", "videos/b"],
        "consumer_address": "consumer:50051"
      }
      ```
    ]
    - `thread_count` is $p$ in the specification.
  ],
  [
    *Output* (JSON logs on stdout):
    - When each upload starts and how it ends
    - Each retry
    - A summary from each thread: sent, accepted, dropped, duplicate, and failed counts
  ],
)

#pagebreak()

#align(center + horizon)[
  = Design Decisions
]

#pagebreak()
== Some Design Decisions
#align(center)[
  #text(size: 14pt)[
    #table(
      columns: (1.3fr, 2.4fr),
      align: (left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Question*], [*Decision*]),
      [When is a drop decided?], [When an upload starts, before any video data is sent],
      [How are large files sent?], [Through a client-streaming RPC. Its metadata comes first, then 1 MiB video fragments],
      [How are duplicates detected?], [A video with the same SHA-256 or a similar perceptual signature. Both are computed by the producer and is included in the metadata],
      [How many connections?], [One `grpc.ClientConn`, shared by all goroutines],
      [What if the consumer is down?], [Retry with exponential backoff on `Unavailable` errors only],
    )
  ]
]

#pagebreak()
== The Queue Belongs to the Consumer
The leaky bucket needs a single place that sees every incoming upload. That place is the consumer, so the producer does not keep a queue of its own.

This affects how the producer treats a drop:

- A drop is an *expected outcome*, not an error. The worker logs and counts it, then continues with the next file.
- A dropped video is *not* sent again. Retrying would make producers wait for space, which is what the leaky bucket avoids.

#pagebreak()
== Streaming the Upload
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 11pt, lines: (2, 6, 7, 8))[
      ```proto
      service VideoUploadService {
        rpc UploadVideo(stream UploadVideoRequest)
          returns (UploadVideoResponse);
      }
      message UploadVideoRequest {
        oneof payload {
          VideoUploadInfo file_info = 1;
          bytes data = 2;
        }
      }
      message VideoUploadInfo {
        string file_name = 1;
        int64 file_size = 2;
        int32 producer_id = 3;
        bytes hash = 4;
        repeated fixed64 signature = 5;
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    By default, gRPC limits inbound messages to 4 MB, so most videos will not fit in one message.

    `oneof` means that each message holds *either* metadata *or* video data. The first message is the metadata, and the rest are chunks.

    Each chunk is 1 MiB, which stays well under the limit.

    The consumer can accept or drop an upload after the first message.
  ],
)

#pagebreak()
== Detecting Duplicates: Two Hashes
#grid(
  columns: (1fr, 1fr),
  gutter: 24pt,
  [
    #set text(size: 16pt)
    *SHA-256* identifies files with identical bytes.

    The *video dHash signature* identifies a video even after it is re-encoded. It stores a 64-bit hash for each second of video, and these stay close to the original's.

    The producer computes both *before* uploading and sends them in the first message, so the consumer can reject a duplicate before receiving any video data.
  ],
  [
    #set text(size: 16pt)
    *Trade-offs:*
    - The producer also needs `ffmpeg` installed.
    - The producer reads each whole file before uploading, even if the upload is later dropped.
    - The signature adds 8 bytes per second of video to the metadata, which is small compared to the 4 MB limit.

  ],
)

#pagebreak()
#consumer(
  [Checking for Duplicates],
  [
    The producer sends both hashes in the first message. The consumer decides what they mean:

    - How similar must two signatures be to count as the same video?
    - How is the check made atomic, so that two identical uploads cannot both pass?
  ],
)

#pagebreak()
== One Connection, Many Streams
#grid(
  columns: (2fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (2, 3))[
      ```go
      func New(address string) (*Client, error) {
        conn, err := grpc.NewClient(address,
          grpc.WithTransportCredentials(insecure.NewCredentials()))
        if err != nil {
          return nil, fmt.Errorf("cannot create client for consumer %s: %w",
            address, err)
        }

        return &Client{
          conn:    conn,
          Service: videouploadv1.NewVideoUploadServiceClient(conn),
        }, nil
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    gRPC runs over HTTP/2, so each upload is a separate *stream* on the same TCP connection. The $p$ goroutines do not need $p$ connections.

    A `ClientConn` is safe to share between goroutines.

    `NewClient` does not connect right away. If the consumer is unreachable, the first RPC fails and the upload is retried.
  ],
)

#pagebreak()
== When to Retry
#align(center)[
  #text(size: 14pt)[
    #table(
      columns: (1.6fr, 1fr, 2.4fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Outcome*], [*Action*], [*Why*]),
      [`codes.Unavailable`], [Retry], [The consumer may come back],
      [`DROPPED_QUEUE_FULL`], [Move on], [This is the leaky bucket's intended behavior],
      [`DROPPED_DUPLICATE`], [Move on], [The consumer already has the video],
      [Any other gRPC error], [Fail the file], [We treat only a lost connection as temporary],
      [Local read or `ffmpeg` error], [Fail the file], [The problem is the file, not the network],
    )
  ]
]

#set text(size: 18pt)
- The delay doubles from 1 s up to 30 s, for at most 15 attempts. This adds up to about 5 minutes of waiting.
- Each retry restarts from *byte 0*, since resuming would require the consumer to keep partial uploads.
#set text(size: 20pt)

#pagebreak()

#align(center + horizon)[
  = Producer Implementation
]

#pagebreak()
== Key Steps
*At startup*
#mermaid("flowchart LR
  A[Read and validate config] --> B[Create shared ClientConn] --> C[Start p goroutines]
", width: 85%)

*For each file in a folder*
#mermaid("flowchart LR
  A[SHA-256] --> B[dHash signature] --> C[Send metadata] --> D[Send 1 MiB chunks] --> E[Final status]
  E -->|Retry on Unavailable| C
")

*When every goroutine finishes*
#mermaid("flowchart LR
  A[Log summaries] --> B[Exit]
", width: 30%)

#pagebreak()
== Startup
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (8, 9, 18, 19))[
      ```go
      func run() error {
        // ... parse the -config flag ...
        pc, err := config.ReadConfigFromJSON(*configPath)
        if err != nil {
          return err
        }

        ctx, stop := signal.NotifyContext(context.Background(),
          os.Interrupt, syscall.SIGTERM)
        defer stop()

        client, err := network.New(pc.ConsumerAddress)
        if err != nil {
          return err
        }
        defer client.Close()

        workers := worker.MakeWorkers(pc.FolderList)
        worker.RunThreads(ctx, workers, client)
        return nil
      }
      ```
    ]],
  [
    `NotifyContext` cancels `ctx` on Ctrl-C or `docker stop`. Every goroutine receives this `ctx`.

    The consumer's address is in the config, so running on separate machines needs no code changes.
  ],
)

#pagebreak()
== Input Validation
#grid(
  columns: (2fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: range(7, 11))[
      ```go
      func (pc ProducerConfig) Validate() error {
        if pc.ThreadCount <= 0 {
          return fmt.Errorf("thread_count must be positive, got %d",
            pc.ThreadCount)
        }

        if len(pc.FolderList) != pc.ThreadCount {
          return fmt.Errorf("thread_count is %d but got %d folders",
            pc.ThreadCount, len(pc.FolderList))
        }

        for _, folder := range pc.FolderList {
          if err := util.CheckDirReadable(folder); err != nil {
            return err
          }
        }

        if err := util.CheckHostPort(pc.ConsumerAddress); err != nil {
          return fmt.Errorf("consumer_address: %w", err)
        }
        // ... the address must also have a host ...
        return nil
      }
      ```
    ]],
  [
    `ReadConfigFromJSON` only returns a valid config, so later code does not check it again.

    Errors are reported before any thread starts.

    Only the address's *format* is checked here. The first RPC shows if it is reachable.
  ],
)

#pagebreak()
== Spawning the Threads
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 11pt, lines: (6, 14))[
      ```go
      func RunThreads(ctx context.Context, pws []ProducerWorker,
        client *network.Client) {
        var wg sync.WaitGroup

        for i := range pws {
          wg.Go(func() {
            if err := pws[i].run(ctx, client); err != nil {
              slog.Error("worker failed", "component", "worker",
                "worker_id", pws[i].id, "error", err)
            }
          })
        }

        wg.Wait()
      }
      ```
    ]],
  [
    #set text(size: 18pt)
    This starts one goroutine per `ProducerWorker`, for $p$ in total. Each has its own ID and folder.

    `wg.Wait()` blocks until all of them finish, so the program cannot exit during an upload.

    If one worker fails, the others keep running.
  ],
)

#pagebreak()
== The Worker Loop
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (4, 5, 16, 17, 18, 20))[
      ```go
      // ... os.ReadDir returns the folder's files, sorted by name ...
      for _, video := range folder {
        // the producer is shutting down, do not start another upload
        if ctx.Err() != nil {
          break
        }

        if video.IsDir() {
          continue
        }

        pw.rs.numSent++
        status, err := upload.File(ctx, client, pw.id,
          filepath.Join(pw.folderName, video.Name()))

        // a failed file should not stop the rest of the folder
        if err != nil {
          pw.rs.numFailed++
          // ... log the failure ...
          continue
        }

        // ... log the status ...
        pw.rs.update(status)
      }
      ```
    ]],
  [
    #set text(size: 18pt)
    Each folder is uploaded one file at a time. The parallelism is *between* folders.

    If a file fails, the worker counts it and continues with the next file.

    The worker skips subfolders.
  ],
)

#pagebreak()
== Preparing an Upload
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (1, 5, 14, 15))[
      ```go
      hash, err := videohash.SHA256(file)
      if err != nil {
        return statusUnknown, fmt.Errorf("upload: %w", err)
      }
      signature, err := videohash.VideoSignature(ctx, file)
      if err != nil {
        return statusUnknown, fmt.Errorf("upload: %w", err)
      }

      info := &videouploadv1.VideoUploadInfo{
        FileName:   filepath.Base(path),
        FileSize:   stats.Size(),
        ProducerId: int32(producerID),
        Hash:       hash,
        Signature:  signature,
      }

      job := uploadJob{client: client, info: info, file: file}
      status, err := job.uploadOrTryAgain(ctx)
      ```
    ]],
  [
    Both hashes are computed outside the retry loop, so retries do not recompute them.

    `uploadJob` holds the values shared by all attempts of an upload.
  ],
)

#pagebreak()
== SHA-256 and the Signature
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (5, 6, 16, 17))[
      ```go
      // extractFrameRTI: raw frames from ffmpeg
      raw, err := ffmpeg.Run(ctx,
        "-v", "error",
        "-i", video.Name(),
        "-vf", fmt.Sprintf("fps=%d,scale=%d:%d:flags=area,format=gray",
          fps, frameWidth, frameHeight),
        "-f", "rawvideo",
        "pipe:1",
      )

      // dhash: one 64-bit hash for each frame
      var hash uint64
      for row := range frameHeight {
        for col := range frameWidth - 1 {
          hash <<= 1
          if frame[row*frameWidth+col] < frame[row*frameWidth+col+1] {
            hash |= 1
          }
        }
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    `ffmpeg` samples one frame per second and shrinks it to $9 times 8$ grayscale pixels.

    Each pixel is compared to its right neighbor, giving $8 times 8 = 64$ bits per frame.

    Re-encoding changes pixel values, but it rarely changes which neighbor is brighter.

    SHA-256 uses a `SectionReader`, so it does not move the file's read position.
  ],
)

#pagebreak()
#consumer(
  [Corrupt and Non-Video Files],
  [
    The producer's `ffmpeg` step already fails on these files, so they never reach the consumer.

    - Is that enough, or should the consumer also receive and report them? (Issue 3.5)
    - How do consumer workers handle a file that passes the signature step but fails later?
  ],
)

#pagebreak()
== Sending the File
#grid(
  columns: (2fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (3, 11))[
      ```go
      // sendFile: metadata first, then the chunks
      err := stream.Send(&videouploadv1.UploadVideoRequest{
        Payload: &videouploadv1.UploadVideoRequest_FileInfo{FileInfo: info},
      })
      if err != nil {
        return err
      }

      return sendChunks(r, func(chunk []byte) error {
        return stream.Send(&videouploadv1.UploadVideoRequest{
          Payload: &videouploadv1.UploadVideoRequest_Data{Data: chunk},
        })
      })
      ```
    ]],
  [
    #set text(size: 18pt)
    `sendChunks` reads up to 1 MiB at a time into one reused buffer, so large videos do not need more memory.

    HTTP/2 flow control makes `Send` wait when the consumer falls behind.
  ],
)

#pagebreak()
== A Single Upload Attempt
#grid(
  columns: (2fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 9pt, lines: (3, 6, 15, 16))[
      ```go
      func (job uploadJob) attemptUpload(ctx context.Context) (
        videouploadv1.VideoUploadStatus, error) {
        ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
        defer cancel()

        if _, err := job.file.Seek(0, io.SeekStart); err != nil {
          return statusUnknown, err
        }

        stream, err := job.client.Service.UploadVideo(ctx)
        if err != nil {
          return statusError, fmt.Errorf("attemptUpload %s: %w", job.file.Name(), err)
        }

        err = sendFile(stream, job.info, job.file)
        if err != nil && !errors.Is(err, io.EOF) {
          return statusError, fmt.Errorf("attemptUpload %s: %w", job.file.Name(), err)
        }

        resp, err := stream.CloseAndRecv()
        // ... return resp.GetStatus() ...
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    Each attempt has its own 10-minute timeout.

    `Seek` returns to the start of the file, so a retry sends the whole video again.

    `Send` returns `io.EOF` if the stream ends early. `CloseAndRecv` then returns the actual result.
  ],
)

#pagebreak()
== The Retry Loop
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (4, 5, 11, 12, 13, 14, 15))[
      ```go
      currDelay := retryDelay
      for attempt := range retryMaxAttempts {
        status, err := job.attemptUpload(ctx)
        if err == nil {
          return status, nil
        } else if !isRetryable(err) {
          return statusError, fmt.Errorf(...)
        }

        // ... stop after the last attempt, log the retry ...
        select {
        case <-time.After(currDelay):
        case <-ctx.Done():
          return statusUnknown, ctx.Err()
        }

        currDelay *= retryDelayMultiplier
        if currDelay > retryMaxDelay {
          currDelay = retryMaxDelay
        }
      }
      ```
    ]],
  [
    #set text(size: 18pt)
    A dropped upload has `err == nil`, so it returns immediately and is not retried.

    The wait uses `select` instead of `time.Sleep`, so a shutdown can interrupt it.
  ],
)

#pagebreak()
== Run Summary
#grid(
  columns: (2fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 9pt, lines: (4, 6, 8, 10, 12))[
      ```go
      func (rs *RunSummary) update(uploadStatus videouploadv1.VideoUploadStatus) {
        switch uploadStatus {
        case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_STATUS_UNSPECIFIED:
          rs.numFailed++
        case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ACCEPTED:
          rs.numAccepted++
        case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_QUEUE_FULL:
          rs.numDropped++
        case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_DROPPED_DUPLICATE:
          rs.numDuplicate++
        case videouploadv1.VideoUploadStatus_VIDEO_UPLOAD_ERROR:
          rs.numFailed++
        }
      }
      ```
    ]],
  [
    #set text(size: 18pt)
    `sent` always equals the sum of the other four counts.

    Each worker has its own `RunSummary`, so the counters do not need a lock.

    Each thread logs its summary when it finishes.
  ],
)

#pagebreak()
#consumer(
  [Consumer Implementation],
  [
    The consumer's key implementation steps:

    - Accepting and staging an upload
    - The $c$ worker threads
    - Saving videos to the media folder
  ],
)

#pagebreak()

#align(center + horizon)[
  = Queueing Details
]

#pagebreak()
== Queue Results on the Producer
The producer cannot access the queue directly. It only receives a status for each upload.

#align(center)[
  #text(size: 14pt)[
    #table(
      columns: (1.8fr, 2fr, 1fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Status*], [*Meaning*], [*Counted as*]),
      [`VIDEO_UPLOAD_ACCEPTED`], [The consumer accepted the video], [`accepted`],
      [`VIDEO_UPLOAD_DROPPED_QUEUE_FULL`], [The queue already holds $q$ videos], [`dropped`],
      [`VIDEO_UPLOAD_DROPPED_DUPLICATE`], [The consumer already has the video], [`duplicate`],
      [`VIDEO_UPLOAD_ERROR`], [The upload failed on the consumer], [`failed`],
    )
  ]
]

Any status other than `ACCEPTED` is logged as a warning.

#pagebreak()
== Dropping an Upload Early
#grid(
  columns: (1fr, 1fr),
  gutter: 24pt,
  [
    #mermaid("sequenceDiagram
      participant P as Producer
      participant C as Consumer
      P->>C: file_info
      Note over C: Queue full?
      C-->>P: DROPPED_QUEUE_FULL
      Note over P,C: No video data is sent
    ")
  ],
  [
    #set text(size: 16pt)
    The metadata is sent first, so the consumer can drop an upload before receiving any video data.

    The producer's next `Send` returns `io.EOF`, and `CloseAndRecv` returns the status.

    *Test:* a test server dropped a 571 MB file 6 ms after its upload started, without reading any video data.

    The producer had already spent about 3.5 s hashing the file. This work is lost when an upload is dropped.
  ],
)

#pagebreak()
#consumer(
  [Admission and the Bounded Queue],
  [
    The queue itself, which is the center of the leaky bucket:

    - How does an upload claim one of the $q$ slots, and when is the slot released?
    - How is the drop decided without blocking?
    - How do the $c$ workers take videos from the queue?
  ],
)

#pagebreak()
#consumer(
  [Signaling a Full Queue (Bonus 5.1)],
  [
    The producer already logs and counts `DROPPED_QUEUE_FULL`, then continues with its next file. It does not call `GetQueueStatus` yet.

    - When does the consumer send this status?
    - What does `GetQueueStatus` report, and should producers use it to slow down?
  ],
)

#pagebreak()
#consumer(
  [Interrupted and Retried Uploads],
  [
    The producer restarts an interrupted upload from byte 0. The consumer needs to define what happens on its side:

    - Is the slot released when an upload dies midway, before the retry arrives?
    - Where do partial files go, and who deletes them?
    - If a video is saved but the response is lost, the retry looks like a duplicate. Is that acceptable?
  ],
)

#pagebreak()

#align(center + horizon)[
  = Producer/Consumer Concepts
]

#pagebreak()
== The Producer-Consumer Pattern
#align(center)[
  #text(size: 14pt)[
    #table(
      columns: (1fr, 1.7fr, 2fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Concept*], [*Classic*], [*This project*]),
      [Producers], [Threads that create items], [$p$ goroutines, each with one folder],
      [Item], [A value in memory], [A video, sent in chunks],
      [Buffer], [Shared memory, size $q$], [Queue on another machine, size $q$],
      [Buffer full], [Producer *blocks*], [The upload is *dropped*, and the producer continues],
      [Signaling], [Semaphores, condition variables], [gRPC status messages],
      [Consumers], [Threads that take items], [$c$ workers on the consumer],
    )
  ]
]

#set text(size: 18pt)
With a leaky bucket, a full queue does not slow the producer down, but dropped videos are not saved.
#set text(size: 20pt)

#pagebreak()

#align(center + horizon)[
  = Synchronization
]

#pagebreak()
== Synchronization in the Producer
#align(center)[
  #text(size: 13pt)[
    #table(
      columns: (1.5fr, 1.2fr, 2.6fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Mechanism*], [*Where*], [*What it guarantees*]),
      [`sync.WaitGroup`], [`RunThreads`], [The program does not exit during an upload],
      [One worker, one folder], [`ProducerWorker`], [Workers do not compete for files, and counters need no lock],
      [`grpc.ClientConn`], [Shared by all], [Concurrent uploads are safe, with one stream each],
      [`context` cancellation], [All blocking work], [A shutdown stops RPCs, `ffmpeg`, and retry waits],
      [`select` on `ctx.Done()`], [Retry wait], [A waiting worker wakes up on shutdown],
      [HTTP/2 flow control], [`stream.Send`], [A slow consumer slows the sender, which limits memory use],
    )
  ]
]

#set text(size: 18pt)
The producer does not need a mutex, because its workers do not share any mutable state.
#set text(size: 20pt)

#pagebreak()
#consumer(
  [Synchronization in the Consumer],
  [
    Most of the project's shared state is here. For each mechanism, state what it protects:

    - The queue and its slots
    - The duplicate index
    - Moving files from staging to the media folder
    - Counters and shutdown
  ],
)

#pagebreak()
== No Deadlock, No Starvation
#grid(
  columns: (1fr, 1fr),
  gutter: 24pt,
  [
    #set text(size: 17pt)
    *No deadlock*
    - Workers hold no locks and do not wait for each other.
    - So, hold-and-wait and circular wait cannot occur between them.
    - `wg.Wait()` is the only join, and the workers never wait for the main goroutine.
  ],
  [
    #set text(size: 17pt)
    *No starvation*
    - A signature takes at most 5 minutes.
    - An upload attempt takes at most 10 minutes.
    - A file gets at most 15 attempts.

    Workers do not depend on each other, so a slow folder cannot starve the others.
  ],
)

#pagebreak()
== Shutdown
#set text(size: 18pt)
Ctrl-C or `docker stop` cancels the `ctx` shared by all goroutines. What happens next depends on what the worker is doing:

- *Between files:* it does not start another upload.
- *During an upload:* the RPC is cancelled and not retried.
- *During a retry wait:* the `select` returns immediately.
- *During signature generation:* `exec.CommandContext` stops `ffmpeg`.
- *During SHA-256:* the hash finishes first, since it cannot be interrupted.

Each worker then logs its summary. An upload in progress is *cancelled* and counted as `failed`.
#set text(size: 20pt)

#pagebreak()

#align(center + horizon)[
  = Results
]

#pagebreak()
== Producer Tests
The consumer is not finished, so we tested the producer against a small test server that implements the `.proto`.

#align(center)[
  #text(size: 13pt)[
    #table(
      columns: (1.7fr, 2.6fr),
      align: (left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Scenario*], [*Observed*]),
      [2 folders: 571 MB and 297 KB], [Both `ACCEPTED`, with 545 chunks of at most 1 MiB, and SHA-256 matches],
      [Server drops the upload after the metadata], [`dropped=1`, and the server read no video data],
      [Server down at the start], [Retries after 1, 2, 4, and 8 s, then `ACCEPTED`],
      [Server crashes after 20 chunks], [Restarts from byte 0, then `ACCEPTED`, and SHA-256 matches],
      [File is not a video], [`ffmpeg` error, `failed=1`, and the worker finishes normally],
      [SIGINT during a retry wait], [Stops immediately, `failed=1`, and exit code 0],
      [`thread_count` 2, one folder], [Config error, and no upload starts],
    )
  ]
]

#pagebreak()
== Sample Run: Server Crash During an Upload
#highlighted-code(size: 9.5pt, lines: (2, 6))[
  ```text
  12:32:56.635 INFO  upload started     producer_id=0 file=a.mp4 size_bytes=571027959
  12:32:56.661 WARN  attempt 1 failed   retry_in=1s   code=Unavailable (error reading from server: EOF)
  12:32:57.663 WARN  attempt 2 failed   retry_in=2s   code=Unavailable (connection error)
  12:32:59.664 WARN  attempt 3 failed   retry_in=4s   code=Unavailable (connection error)
  12:33:02.103 INFO  upload finished    worker_id=0 file=a.mp4 status=VIDEO_UPLOAD_ACCEPTED
  12:33:02.103 INFO  worker finished    worker_id=0 sent=1 accepted=1 dropped=0 duplicate=0 failed=0
  ```
]

#text(size: 16pt)[
  - The test server exits after 20 chunks and restarts about 3 s later. The log lines are shortened.
  - After the restart, the server received all 571,027,959 bytes, with a matching SHA-256.
  - The summary shows `sent=1`, since retries count as part of the same upload.
]

#pagebreak()
#consumer(
  [End-to-End Results],
  [
    To fill in once the consumer is running:

    - Do the producer's summaries match the consumer's counters? (Issue 1.6)
    - Stress test with $q = 1$, $c = 1$, and $p > q$. (Issue 2.9)
    - A sample run against the real queue, with real drops.
  ],
)

#pagebreak()
#consumer(
  [Deployment],
  [
    Neither program has a Dockerfile yet. (Issue 0.5)

    - Both images need `ffmpeg`, since the producer now computes the signature.
    - How are the input and output folders mounted?
    - Must the containers run on separate VMs? (Ambiguity 9)
  ],
)

#pagebreak()
== Summary & Conclusion
- The producer runs one goroutine per folder, all sharing one process and one gRPC connection.
- The queue is on the consumer, so the producer counts a drop as an outcome and does not retry it.
- Sending the metadata first lets the consumer drop an upload before any video data is sent.
- Workers share no mutable state, so the producer needs no mutex. `context` handles shutdown.
