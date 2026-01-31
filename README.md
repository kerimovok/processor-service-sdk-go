# Processor Service SDK

A Go SDK for interacting with the processor-service microservice. This SDK provides a type-safe client for events, scripts, and script executions (list, get, create, update, delete where applicable).

## Installation

```bash
go get github.com/kerimovok/processor-service-sdk-go
```

## Features

- **Type-safe**: Full type definitions for list/get/create/update responses (events, scripts, script executions)
- **Error handling**: `APIError` and `IsAPIError()` for API-level errors
- **Pagination**: List endpoints support raw query string forwarding (page, per_page, filters)

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"

    processorsdk "github.com/kerimovok/processor-service-sdk-go"
)

func main() {
    client, err := processorsdk.NewClient(processorsdk.Config{
        BaseURL: "http://localhost:3004",
        Timeout: 10 * time.Second,
    })
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // List events
    resp, err := client.ListEvents(ctx, "page=1&per_page=20")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Events: %d items\n", len(resp.Data))

    // List scripts
    scripts, err := client.ListScripts(ctx, "")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Scripts: %d items\n", len(scripts.Data))
}
```

## API Reference

### Events

- **ListEvents(ctx, queryString)** – Paginated list (query string forwarded as-is)
- **GetEvent(ctx, id)** – Get an event by ID
- **UpdateEvent(ctx, id, payload)** – Update event payload
- **DeleteEvent(ctx, id)** – Delete an event

### Scripts

- **ListScripts(ctx, queryString)** – Paginated list
- **GetScript(ctx, id)** – Get a script by ID
- **CreateScript(ctx, body)** – Create a script
- **UpdateScript(ctx, id, body)** – Update a script
- **DeleteScript(ctx, id)** – Delete a script
- **ListScriptExecutionsByScriptID(ctx, scriptID, queryString)** – List executions for a script

### Script Executions

- **ListScriptExecutions(ctx, queryString)** – Paginated list
- **GetScriptExecution(ctx, id)** – Get a script execution by ID

## Configuration

- **BaseURL**: Processor service base URL (e.g. `http://localhost:3004`)
- **Timeout**: Request timeout (optional, default 10s)

## Error Handling

```go
resp, err := client.GetEvent(ctx, id)
if err != nil {
    if apiErr, ok := processorsdk.IsAPIError(err); ok {
        fmt.Printf("API Error (status %d): %s\n", apiErr.StatusCode, apiErr.Message)
    }
    return err
}
```

## License

MIT
