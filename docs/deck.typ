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
    - Each skipped non-video filed failed counts
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
      [What if the consumer is down?], [Retry with exponential backoff on `Unavailable` and `Aborted` errors only], [What happens after an interruption?], [Ask the consumer for its offset and resume]

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
        rpc UploadVideo(stream UploadVideoRequest) returns (UploadVideoResponse); 
        rpc GetQueueStatus(GetQueueStatusRequest) returns (GetQueueStatusResponse);
        rpc GetUploadOffset(GetUploadOffsetRequest) returns (GetUploadOffsetResponse);
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
        int64 offset = 6;
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    By default, gRPC limits inbound messages to 4 MB, so most videos will not fit in one message.

    `oneof` means that each message holds *either* metadata *or* video data. The first message is the metadata, and the rest are chunks.

    Each chunk is 1 MiB, which stays well under the limit.

    The consumer can accept or drop an upload after the first message.

    Offset tells the consumer where the chunks start.
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
== Checking for Duplicates
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (2, 5, 9, 10, 15))[
      ```go
      func (idx *Index) CheckAndInsert(id string, signature []uint64) bool {
        idx.mu.Lock()
        defer idx.mu.Unlock()

        if _, exists := idx.signatures[id]; exists {
          return true
        }

        for _, existing := range idx.signatures {
          if isDuplicate(signature, existing) {
            return true
          }
        }

        idx.signatures[id] = slices.Clone(signature)
        return false
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    The key is the video's SHA-256 in hex, so an identical file matches right away. Otherwise the signature is compared with every stored one.

    The check and the insert happen under *one lock*, so two identical uploads cannot both pass.

    A signature is stored when the upload starts, so videos still uploading count too. It is removed if the upload or the job fails.
  ],
)

#pagebreak()
== How Similar Is "The Same Video"?
#align(center)[
  #text(size: 14pt)[
    #table(
      columns: (1.5fr, 0.5fr, 2.6fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Setting*], [*Value*], [*Meaning*]),
      [`maxFrameDistance`], [10], [Two frames match if at most 10 of their 64 bits differ],
      [`minInformativeBits`], [4], [A frame with fewer than 4 ones or 4 zeros, like a black screen, never matches],
      [`minMatchRatio`], [0.7], [At least 70% of the informative frames must match],
    )
  ]
]

#set text(size: 18pt)
- `bestAlignment` slides one signature along the other and keeps the offset with the most matches, so a trimmed start still matches.
- The scan visits every stored signature while holding the lock, which is fine at this scale.
#set text(size: 20pt)

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
      [`codes.Aborted`], [Retry], [The offset we used was out of date, so the next attempt asks again],
      [Any other gRPC error], [Fail the file], [Only a lost connection or a stale offset  is temporary],
    )
  ]
]

#set text(size: 18pt)
- The delay doubles from 1 s up to 30 s, for at most 15 attempts. This adds up to about 5 minutes of waiting.
- Each retry checks GetUploadOffset() in order to start sending from the remaining bytes.
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
  A[SHA-256] --> B[dHash signature] --> C[Check offset] --> D[Send metadata] --> E[Send 1 MiB chunks] --> F[Final status]

  E -->|Retry on Unavailable or Aborted| B
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
        
        if !videoExts[strings.ToLower(filepath.Ext(video.Name()))] {
          // ... log the skipped file ...
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
== Corrupt and Non-Video Files
#align(center)[
  #text(size: 14pt)[
    #table(
      columns: (1.6fr, 0.9fr, 2.5fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Situation*], [*Where*], [*What happens*]),
      [Extension is not a video type, like `notes.txt`], [Producer], [Skipped and logged. It is not counted in `sent` and nothing is uploaded],
      [Video extension, but corrupt or not really a video], [Producer], [`ffmpeg` fails while computing the signature. The file is counted as `failed`, is not retried, and never reaches the consumer],
      [Upload ends early or has the wrong size], [Consumer], [The `.part` file is deleted and the consumer returns `InvalidArgument`. The producer counts it as `failed`],
      [Passes the signature step but compression fails], [Consumer], [`ffmpeg` fails, times out, or is cancelled, so the original is saved as is],
    )
  ]
]

#set text(size: 18pt)
- The producer is the main filter. Its extension check and `ffmpeg` step stop bad files before any video data is sent.
- The consumer does not check extensions, so a bad file that reached it would be saved uncompressed.
#set text(size: 20pt)

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

        rOffset, err := job.client.Service.GetUploadOffset(ctx, &videouploadv1.GetUploadOffsetRequest{Hash: job.info.Hash})
    
      	if err != nil {
      		return statusError, fmt.Errorf("attemptUpload %s: %w", job.file.Name(), err)
      	}
      
      	offset := rOffset.GetOffset()
      
      	if _, err := job.file.Seek(offset, io.SeekStart); err != nil {
      		return statusUnknown, err
      	}
      
      	job.info.Offset = offset
      
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

  `GetUploadOffset` means that each retry starts from where it left off.n.

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
== Startup and Shutdown
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (1, 4, 7, 8, 9))[
      ```go
      _, err = server.CleanStaging(cc.StagingFolder)
      // ... listen, build q, idx, h, mut ...
      // ... start the gRPC server and the workers ...

      srv.GracefulStop()
      q.Close()
      <-done
      ```
    ]],
  [
    #set text(size: 16pt)
    *Startup:* config, `CleanStaging`, listen, shared state, server, then workers.

    *Shutdown order matters:*
    + `GracefulStop`: running uploads finish.
    + `q.Close`: after step 1, since enqueueing to a closed queue panics.
    + `<-done`: the workers drain the queue. After Ctrl+C they skip compression and save the originals.
  ],
)

#pagebreak()
== Consumer Key Steps
*For each upload*
#mermaid("flowchart LR
  A[Metadata] --> B[Duplicate check] --> C[Claim hash] --> D[Reserve slot] --> E[Append to .part] --> F[Queue job]
")

#pagebreak()
== Resuming a Partial File
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (2, 7, 8, 9, 10, 14))[
      ```go
      // no O_TRUNC: a resume keeps the bytes already there
      f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY, 0o644)
      // ... Stat the file ...
      tempOffset := st.Size()

      switch {
      case tempOffset < info.Offset:
        return status.Error(codes.Aborted, "offset mismatch")
      case tempOffset > info.Offset:
        f.Truncate(info.Offset)
        tempOffset = info.Offset
      }

      f.Seek(info.Offset, io.SeekStart)
      // ... append each chunk, then check the final size ...
      ```
    ]],
  [
    #set text(size: 16pt)
    Each upload writes to `upload-<hash>.part`. The name depends only on the SHA-256, so every attempt for the same video finds the same file.

    `GetUploadOffset` returns the size of that file, or 0 if it does not exist. The producer sends that number as `offset` in its metadata.

    The consumer then compares the file's size with `offset`:
    - *Equal:* append from there.
    - *File shorter:* the producer's offset is out of date. The consumer returns `Aborted`, and the producer asks again.
    - *File longer:* it cuts the file back to `offset`, since the producer is about to resend those bytes.

    A file with the wrong final size is deleted, so a bad `.part` is never resumed.
  ],
)

#pagebreak()
== Saving a Video
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (1, 3, 9, 14))[
      ```go
      keep, discard := c.pick(jobCtx, j.StagingPath, compressPath)

      c.mut.Lock()
      defer c.mut.Unlock()

      // ... choose "name.ext", then "name (1).ext", and so on,
      //     until a name is free in the media folder ...

      if err := os.Rename(keep, dst); err != nil {
        os.Remove(keep)
        os.Remove(discard)
        return fmt.Errorf(...)
      }
      os.Remove(discard)
      ```
    ]],
  [
    #set text(size: 16pt)
    Compression runs *before* the lock, so workers compress in parallel.

    The mutex covers only the name check and the rename, so two workers cannot pick the same name.

    `os.Rename` is atomic, so nobody sees a half-written video. The other copy is deleted only after it succeeds.
  ],
)

*For each job, on one of $c$ workers*
#mermaid("flowchart LR
  A[Take job, free slot] --> B[Compress with ffmpeg] --> C[Keep the smaller file] --> D[Rename into media folder]
")

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
      [`VIDEO_UPLOAD_ERROR`], [Reserved. The consumer reports failures as gRPC errors.], [`failed`],
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
      P->>C: GetUploadOffset
      C-->>P: offset
      P->>C: file_info
      Note over C: Duplicate? Queue full?
      C-->>P: DROPPED_DUPLICATE or DROPPED_QUEUE_FULL
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
== The Bounded Queue
#grid(
  columns: (1.5fr, 1fr),
  gutter: 24pt,
  [
    #highlighted-code(size: 10pt, lines: (3, 5, 12))[
      ```go
      func (q *Queue) TryReserve() bool {
        select {
        case q.reserve <- struct{}{}:
          return true
        default:
          return false
        }
      }

      // the worker calls this when it takes a job
      func (q *Queue) UnReserve() {
        <-q.reserve
      }
      ```
    ]],
  [
    #set text(size: 16pt)
    The queue is two channels of size $q$: `reserve` holds one *permit* per upload, and `ch` holds the finished jobs.

    - *Claim:* an upload takes a permit at its first message, before any data.
    - *Release:* when a worker takes the job, or when the upload fails.
    - *No blocking:* `select` with `default` fails at once when no permit is free. That is the drop.
    - *Workers:* each reads `ch`, and each job goes to exactly one worker.

    A job only enters `ch` while holding a permit, so `TryEnqueue` cannot fail.
  ],
)

#pagebreak()
== Signaling a Queue is Full
The producer cannot access the queue directly. It only receives a status for each upload.
- `UploadVideo` replies `DROPPED_QUEUE_FULL` when `TryReserve` fails, after the duplicate and in-flight checks.
- `GetQueueStatus` returns `MaxSize` and `CurrSize`, the permits in use, including uploads still arriving.
- It is only a snapshot, so a free slot can be gone by the next upload.
- The producer does not call it yet. It only reacts to the status of each upload.

#pagebreak()
== Interrupted and Retried Uploads
- Upload dies midway: the slot, hash and index entry are released, and the .part file is kept.
- Producer asks for its offset: GetUploadOffset returns the .part size, or 0.
- `.part` shorter than the producer’s offset: Aborted, so the producer asks again.
- `.part` longer than the offset: truncate to the offset.
- Final size wrong: delete the .part and return InvalidArgument.
- Producer never returns: CleanStaging removes .part files older than 24 h at startup.

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
== Synchronization in the Consumer
#align(center)[
  #text(size: 13pt)[
    #table(
      columns: (1.5fr, 1.2fr, 2.6fr),
      align: (left, left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Mechanism*], [*Where*], [*What it guarantees*]),
      [`reserve` channel, non-blocking send], [`Queue`], [Never more than $q$ slots, and a drop never blocks],
      [Job channel], [`Queue`], [Each job goes to exactly one worker],
      [`Index.mu`], [`dedup`], [Check and insert are atomic, so two identical uploads cannot both pass],
      [`Hashbrowns.mut`], [`inflight`], [Only one stream writes a given video],
      [Name mutex], [`worker`], [Two workers cannot pick the same file name],
      [`os.Rename`], [`worker`], [A video appears in the media folder whole],
      [Close queue after `GracefulStop`], [`main`], [Workers drain the queue before exit],
    )
  ]
]

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
Producer is tested against a small test server that implements the `.proto`.

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
      [Server crashes after 20 chunks], [Resume from about 20 MiB and still match the SHA-256A-256 matches],
      [Corrupt file with a video extension], [`ffmpeg` error, `failed=1`, and the worker finishes normally],
      [`notes.txt` in a folder],[Skipped and logged, and `sent` does not change],
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
== Consumer Tests
#align(center)[
  #text(size: 13pt)[
    #table(
      columns: (1.7fr, 2.6fr),
      align: (left, left),
      stroke: table-stroke,
      fill: table-fill,
      table.header([*Scenario*], [*Observed*]),
      [Invalid config values], [Returns specific errors for zero limits and malformed addresses],
      [Queue exceeds capacity], [Queue drops new items instead of blocking (`TryEnqueueDropsWhenFull`)],
     [Dedup window logic], [Correctly calculates frame distance, alignment, and identifies duplicates],
      [Concurrent dedup access], [Concurrent check/insert operations pass without race conditions],
      [Dedup index management], [Check, insert, and remove operations maintain consistent state],
    )
  ]
]

#pagebreak()
== Deployment
- Both producer and consumer require ffmpeg
- The  `staging` and `media` folders must be on the same file system

#pagebreak()
== GUI: Consumer Side
- Simple HTML interface displaying videos uploaded by the producer.
- JavaScript interactions:
  - *Hover* over a video box to play the first *10 seconds*.
  - *Click* to open the video in *full screen*.
  - *Press `Esc`* to exit full screend.

#pagebreak()
== Consumer Viewing Server
- Small Go web service for viewing videos inside `./data/media`.
- Reads videos from the media directory and returns metadata as *JSON* for the web page.

#pagebreak()
== `listVideos()` Pipeline
1. *Read directory:* Scans `./data/media` for video files.
2. *Sort chronologically:* Checks each file's modification time and sorts *oldest first* (assumes worker processes move finished videos into the folder only when ready to play).
3. *Clean title & safe URLs:* Strips file extensions (such as `.mp4`) for a clean *Title*, and URL-escapes filenames so characters like spaces, `\#`, or `?` do not break `/data/media/...` links.

#pagebreak()
== `listVideos()` Implementation
#highlighted-code(size: 10pt)[
  ```go
  func listVideos(dir string) ([]VideoInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type file struct {
		name string
		mod  time.Time
	}
	var files []file
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // removed since ReadDir
		}
		files = append(files, file{e.Name(), info.ModTime()})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })

	videos := make([]VideoInfo, 0, len(files))
	for _, f := range files {
		videos = append(videos, VideoInfo{
			Title: strings.TrimSuffix(f.name, filepath.Ext(f.name)),
			// escaped, so names with spaces, "#" or "?" still work
			URL: "/data/media/" + url.PathEscape(f.name),
		})
	}
	return videos, nil
}
```
]

#pagebreak()
== Web Routes & Server Settings

    /webapp/... — Serves index.html and index.js from the filesystem with the /webapp/ prefix.

    /data/media/... — Streams video files directly from the media folder.

    /api/videos/ — Calls listVideos(), marshals []VideoInfo to JSON, and returns Content-Type: application/json.

    Timeouts: 20s read timeout and 80s idle timeout; WriteTimeout is intentionally unset so long video streams are not cut off mid-playback.

#pagebreak()
== Server Entrypoint (main())
#highlighted-code(size: 10pt)[
```
func main() {
	addr := flag.String("addr", ":8008", "host:port to serve the GUI on")
	media := flag.String("media", "./data/media", "the consumer's media folder")
	flag.Parse()

	mux := http.NewServeMux()

	mux.Handle("/webapp/", http.StripPrefix("/webapp/", http.FileServerFS(static)))
	mux.Handle("/data/media/", http.StripPrefix("/data/media/", http.FileServer(http.Dir(*media))))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, static, "index.html")
	})

	mux.HandleFunc("/api/videos/", func(w http.ResponseWriter, r *http.Request) {
		videos, err := listVideos(*media)
		if err != nil {
			http.Error(w, "unable to read video directory", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(videos)
	})

	server := &http.Server{
		Addr:        *addr,
		Handler:     mux,
		ReadTimeout: 20 * time.Second,
		IdleTimeout: 80 * time.Second,
		// no WriteTimeout: it would cut off a long video while it plays
	}

	log.Printf("GUI on %s, media folder %s", server.Addr, *media)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed to start: %v", err)
	}
}
```
]

#pagebreak()
== Summary & Conclusion
- The producer runs one goroutine per folder, all sharing one process and one gRPC connection.
- The queue is on the consumer, so the producer counts a drop as an outcome and does not retry it.
- Sending the metadata first lets the consumer drop an upload before any video data is sent.
- Workers share no mutable state, so the producer needs no mutex. `context` handles shutdown.
- An interrupted upload resumes from the consumer's saved offset, so a retry does not resend bytes the consumer already has.
- Uploads start in their own respective folders in `videos` and end in a `data/media` folder