---
title: Expose data over TCP/IPC
weight: 6
---

Use this guide when non-HTTP clients (CLI tools, sidecars, scripts) need to
perform the same CRUD operations available over HTTP.

## Solution

Enable the TCP (or IPC) transport plus the `data.bridge` module. The bridge
registers `data.list`, `data.get`, `data.create`, `data.update`, and
`data.delete` commands on the transport's command router.

```toml
[modules]
tcp            = true
"data.bridge"  = true

[tcp]
addr  = ":9090"
codec = "json-lines"   # or "length-prefix"
```

Connect with any line-oriented TCP client and send command envelopes:

```
$ nc localhost 9090
{"command":"data.list","payload":{"collection":"posts"}}
{"command":"data.get","payload":{"collection":"posts","id":"hello-world"}}
{"command":"data.create","payload":{"collection":"posts","body":{"id":"new","title":"Hi"}}}
{"command":"data.update","payload":{"collection":"posts","id":"hello-world","body":{"title":"Updated"}}}
{"command":"data.delete","payload":{"collection":"posts","id":"hello-world"}}
```

The bridge reuses the file data module, so the same files served over HTTP
are visible over TCP.

## Variations

### Unix socket instead of TCP

```toml
[modules]
ipc           = true
"data.bridge" = true

[ipc]
path  = "/tmp/myapp.sock"
codec = "json-lines"
```

```
$ nc -U /tmp/myapp.sock
{"command":"data.list","payload":{"collection":"posts"}}
```

### Stream a large list

Add `"stream": true` to a list payload. The bridge replies with envelope
messages instead of a single result:

```json
{"command":"data.list.start","payload":{"collection":"posts","total":2}}
{"command":"data.list.item","payload":{"ID":"hello",...}}
{"command":"data.list.item","payload":{"ID":"goodbye",...}}
{"command":"data.list.end","payload":{"collection":"posts","count":2}}
```

### Binary length-prefix framing

For non-JSON clients, switch the codec:

```toml
[tcp]
codec = "length-prefix"
```

Each frame is a `uint32` big-endian length followed by the JSON payload.

## See also

- [Stream large lists as NDJSON]({{< relref "streaming" >}})
- [Serve markdown content from files]({{< relref "file-content" >}})
