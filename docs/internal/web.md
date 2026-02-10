# internal/web

Embedded web dashboard for Plasma Shield.

## Overview

The web package provides an embedded web UI using Go's `embed` directive. Static files are compiled into the binary, eliminating external file dependencies for the dashboard.

## Structure

Files are located at `internal/web/` in the repository:

```
embed.go           # Go embed directive and handler
static/            # Static web assets
└── index.html     # Single-file Alpine.js dashboard
```

## Code

```go
package web

import (
    "embed"
    "io/fs"
    "net/http"
)

//go:embed static
var staticFiles embed.FS

// Handler returns an http.Handler that serves the embedded web UI.
func Handler() http.Handler {
    // Strip the "static" prefix from the embedded filesystem
    sub, err := fs.Sub(staticFiles, "static")
    if err != nil {
        panic(err)
    }
    return http.FileServer(http.FS(sub))
}
```

## Function

### Handler

Returns an `http.Handler` that serves the embedded static files.

```go
func Handler() http.Handler
```

The handler:
- Strips the `static/` prefix from paths
- Serves files directly from the embedded filesystem
- Works like a standard file server

## Usage

### Basic Integration

```go
import "github.com/Extra-Chill/plasma-shield/internal/web"

mux := http.NewServeMux()

// Mount web UI at root
mux.Handle("/", web.Handler())

// Or mount at a specific path
mux.Handle("/dashboard/", http.StripPrefix("/dashboard", web.Handler()))
```

### With API Server

```go
mux := http.NewServeMux()

// API endpoints
mux.Handle("/api/status", statusHandler)
mux.Handle("/api/agents", agentsHandler)
mux.Handle("/api/rules", rulesHandler)

// Web UI for everything else
mux.Handle("/", web.Handler())

http.ListenAndServe(":8080", mux)
```

### Full Server Example

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/Extra-Chill/plasma-shield/internal/api"
    "github.com/Extra-Chill/plasma-shield/internal/web"
)

func main() {
    // Create API server
    apiCfg := api.ServerConfig{
        Addr:            ":9000",
        ManagementToken: "secret",
        Version:         "1.0.0",
    }
    
    // Create combined mux
    mux := http.NewServeMux()
    
    // API routes (handled by api package internally)
    // ...
    
    // Web UI
    mux.Handle("/", web.Handler())
    
    log.Println("Starting server on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

## Embedding Static Files

The `//go:embed static` directive embeds all files in the `static/` directory into the binary at compile time.

### Adding Files

Place files in `internal/web/static/`:

```
internal/web/static/
└── index.html    # Currently a single-file Alpine.js app
```

All files are automatically included in the build.

### Accessing Files

Files are served relative to the embedded `static/` directory:

| File Location | URL Path |
|---------------|----------|
| `internal/web/static/index.html` | `/` or `/index.html` |

## Benefits

1. **Single Binary**: No external files needed
2. **Immutable**: Dashboard version matches binary version
3. **Simple Deployment**: Copy one file to deploy
4. **No Path Issues**: Files always found regardless of working directory

## Build Requirements

Requires Go 1.16+ for `embed` support.

```go
import "embed"
```

## Package Dependencies

```
web
├── embed   - Go 1.16+ file embedding
├── io/fs   - Filesystem abstraction
└── net/http - HTTP handler
```
