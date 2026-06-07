---
title: Add a custom formatter
weight: 9
---

Use this guide when the built-in formatters (JSON, HTML, text, ANSI) don't
cover what you need — for example, XML, CSV, or MessagePack.

## Solution

Implement the `Formatter` interface and register it on the response pipeline
during your module's `Init`.

```go
package xmlfmt

import (
    "context"
    "encoding/xml"

    "github.com/frob/nullspace/core/response"
    "github.com/frob/nullspace/kernel"
)

type Formatter struct{}

func (f *Formatter) Name() string        { return "xml" }
func (f *Formatter) ContentType() string { return "application/xml" }

func (f *Formatter) Format(ctx context.Context, resp *response.Response) ([]byte, error) {
    return xml.Marshal(resp.Data)
}

type Module struct{}

func New() *Module                                { return &Module{} }
func (m *Module) Name() string                    { return "format.xml" }
func (m *Module) Start(ctx context.Context) error { return nil }
func (m *Module) Stop(ctx context.Context) error  { return nil }

func (m *Module) Init(k *kernel.Kernel) error {
    pipeline, err := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
    if err != nil {
        return err
    }
    pipeline.RegisterFormatter(&Formatter{})
    return nil
}
```

Register the module after the response pipeline:

```go
k.Use(response.NewPipeline())
k.Use(xmlfmt.New())
```

The formatter is now picked when:

- A route sets `format = "xml"`, or
- The request URL has `?format=xml`, or
- The client sends `Accept: application/xml`.

## Variations

### Streaming formatter

Also implement `StreamFormatter` to support `?stream=true`:

```go
func (f *Formatter) StreamContentType() string { return "application/xml" }

func (f *Formatter) FormatStream(
    ctx context.Context, w io.Writer, sr *response.StreamResponse,
) error {
    // Write items incrementally.
}
```

### Replace a built-in formatter

`RegisterFormatter` replaces any previously-registered formatter with the
same `Name()`. Register your own `json` to override the default.

## See also

- [Force a response format on a route]({{< relref "format-override" >}})
- [Stream large lists as NDJSON]({{< relref "streaming" >}})
