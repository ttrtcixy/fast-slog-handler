[English](README.md) | [Русский](README.ru.md)

# High-Performance Slog Handler

The library provides optimized handlers for the slog package. The main focus is on speed and ease of local development.
* **Text Handler**: Created for local development. ANSI highlighting of levels, timestamps, and metadata. (Not recommended for production).
* **JSON Handler**: High-performance handler for production environments.

## Key Features
* Using `sync.Pool` minimizes the load on GC and memory allocation in the heap.
* Optional buffering via `bufio` with background data flushing to reduce latency on system calls.
* Simple transfer of TraceID or RequestID directly via `context.Context`.
* Full thread safety.

## Installation
```shell
go get github.com/ttrtcixy/color-slog-handler
```

## Usage

```go
package main

import (
	"context"
	"log/slog"
	"os"

	logger "github.com/ttrtcixy/color-slog-handler"
)

func main() {
	cfg := &logger.Config{
		Level:          int(slog.LevelDebug),
		BufferedOutput: true, // Enables buffered output and background flushing.
	}

	// Use logger.NewTextHandler(os.Stdout, cfg) for local development
	handler := logger.NewJsonHandler(os.Stdout, cfg)
	l := slog.New(handler)

	// Important: Close the handler to stop the background flusher and flush remaining logs.
	defer handler.Close(context.Background())

	l.LogAttrs(nil, slog.LevelInfo, "msg", slog.String("key", "val"))

	// Inject attributes into the context
	ctx := handler.AppendAttrsToCtx(context.Background(), slog.String("trace_id", "af82-bx22"))

	// The logger will automatically extract and include these attributes
	l.LogAttrs(ctx, slog.LevelInfo, "msg")
}
```

## Configuration
The `Config` struct supports environment variables via tags:
* `Level`: Logging level (e.g., Debug=-4, Info=0).
* `BufferedOutput`: Enable/Disable 4 KB buffer with automatic periodic flushing.

## Important Note on Buffering
If `BufferedOutput` is set to: true, you must call `handler.Close(ctx)`:
* It stops the background flushing goroutine.
* It ensures that all remaining logs in the 4096-byte buffer are written to the output.

Calling `Close()` for an unbuffered handler will return `ErrNothingToClose`.

## Roadmap
* Support for `slog.LogValuer`.
