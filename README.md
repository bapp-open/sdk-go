# BAPP Auto API Client — Go

Official Go client for the [BAPP Auto API](https://www.bapp.ro). Provides a
simple, consistent interface for authentication, entity CRUD, and task execution.

Zero dependencies — uses only the standard library.

## Getting Started

### 1. Install

```bash
go get github.com/bapp-open/sdk-go
```

### 2. Create a client

```go
import bapp "github.com/bapp-open/sdk-go"

client := bapp.NewClient(bapp.WithToken("your-api-key"))
```

### 3. Make your first request

```go
// List with filters
countries, _ := client.List("core.country", url.Values{"page": {"1"}})

// Get by ID
country, _ := client.Get("core.country", "42")

// Create
data := map[string]interface{}{"name": "Romania", "code": "RO"}
created, _ := client.Create("core.country", data)

// Patch (partial update)
client.Patch("core.country", "42", map[string]interface{}{"code": "RO"})

// Delete
client.Delete("core.country", "42")
```

## Authentication

The client supports **Token** (API key) and **Bearer** (JWT / OAuth) authentication.
Token auth already includes a tenant binding, so you don't need to specify `tenant` separately.

```go
// Static API token (tenant is included in the token)
client := bapp.NewClient(bapp.WithToken("your-api-key"))

// Bearer (JWT / OAuth)
client := bapp.NewClient(bapp.WithBearer("eyJhbG..."), bapp.WithTenant("1"))
```

## Configuration

`tenant` and `app` can be changed at any time after construction:

```go
client.Tenant = "2"
client.App = "wms"
```

## API Reference

### Client options

| Option | Description | Default |
|--------|-------------|---------|
| `token` | Static API token (`Token <value>`) — includes tenant | — |
| `bearer` | Bearer / JWT token | — |
| `host` | API base URL | `https://panel.bapp.ro/api` |
| `tenant` | Tenant ID (`x-tenant-id` header) | `None` |
| `app` | App slug (`x-app-slug` header) | `"account"` |
| `timeout` | HTTP request timeout (seconds) | `30` |
| `max_retries` | Max retries on transient errors (5xx, 429, connection) | `3` |

### Methods

| Method | Description |
|--------|-------------|
| `me()` | Get current user profile |
| `get_app(app_slug)` | Get app configuration by slug |
| `list(content_type, **filters)` | List entities (paginated) |
| `get(content_type, id)` | Get a single entity |
| `create(content_type, data)` | Create an entity |
| `update(content_type, id, data)` | Full update (PUT) |
| `patch(content_type, id, data)` | Partial update (PATCH) |
| `delete(content_type, id)` | Delete an entity |
| `list_introspect(content_type)` | Get list view metadata |
| `detail_introspect(content_type)` | Get detail view metadata |
| `get_document_views(record)` | Extract available views from a record |
| `get_document_url(record, output?, label?, variation?)` | Build a render/download URL |
| `get_document_content(record, output?, label?, variation?)` | Fetch document bytes (PDF, HTML, JPG) |
| `download_document(record, dest, output?, label?, variation?)` | Stream document to file (memory-efficient) |
| `list_tasks()` | List available task codes |
| `detail_task(code)` | Get task configuration |
| `run_task(code, payload?)` | Execute a task |
| `run_task_async(code, payload?)` | Run a long-running task and poll until done |

### Paginated responses

`list()` returns the results directly as a list/array. Pagination metadata is
available as extra attributes:

- `count` — total number of items across all pages
- `next` — URL of the next page (or `null`)
- `previous` — URL of the previous page (or `null`)

## File Uploads

When data contains file objects, the client automatically switches from JSON to
`multipart/form-data`. Mix regular fields and files in the same call:

```go
// Pass bapp.File values — the client auto-switches to multipart/form-data
f, _ := os.Open("report.pdf")
defer f.Close()

client.Create("myapp.document", map[string]interface{}{
    "name": "Report",
    "file": bapp.File{Name: "report.pdf", Reader: f},
})
```

## Document Views

Records may include `public_view` and/or `view_token` fields with JWT tokens
for rendering documents (invoices, orders, reports, etc.) as HTML, PDF, or images.

The SDK normalises both formats and builds the correct URL automatically:

```go
order, _ := client.Get("company_order.order", "42")

// Get a PDF download URL (auto-detects public_view vs view_token)
url := client.GetDocumentURL(order, "pdf", "", "")

// Pick a specific view by label
url = client.GetDocumentURL(order, "html", "Comanda interna", "")

// Use a variation
url = client.GetDocumentURL(order, "pdf", "", "v4")

// Fetch the actual content as bytes
pdfBytes, _ := client.GetDocumentContent(order, "pdf", "", "")
os.WriteFile("order.pdf", pdfBytes, 0644)

// Enumerate all available views
views := bapp.GetDocumentViews(order)
for _, v := range views {
    fmt.Println(v.Label, v.Type, v.Variations)
}
```

`get_document_views()` returns a list of normalised view entries with `label`,
`token`, `type` (`"public_view"` or `"view_token"`), `variations`, and
`default_variation`. Use it to enumerate available views (e.g. for a dropdown).

## Tasks

Tasks are server-side actions identified by a dotted code (e.g. `myapp.export_report`).

```go
tasks, _ := client.ListTasks()

cfg, _ := client.DetailTask("myapp.export_report")

// Run without payload (GET)
result, _ := client.RunTask("myapp.export_report", nil)

// Run with payload (POST)
result, _ := client.RunTask("myapp.export_report", map[string]interface{}{"format": "csv"})
```

### Long-running tasks

Some tasks run asynchronously on the server. When triggered, they return an `id`
that can be polled via `bapp_framework.taskdata`. Use `run_task_async()` to
handle this automatically — it polls until `finished` is `true` and returns the
final task data (which includes a `file` URL when the task produces a download).

## License

MIT
