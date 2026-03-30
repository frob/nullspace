Responses
=========

The response pipeline handles format resolution, serialization, and writing
HTTP responses.

Response Type
-------------

Handlers build a ``Response`` value and pass it to the pipeline:

.. code-block:: go

    resp := response.NewResponse(http.StatusOK, myData)
    return pipeline.Write(ctx.Context(), ctx.Writer, resp)

The ``Response`` struct:

.. code-block:: go

    type Response struct {
        Status   int               // HTTP status code (defaults to 200)
        Headers  map[string]string // Additional HTTP headers
        Data     any               // Payload to serialize
        Template string            // Template name (for HTML)
        Error    error             // Triggers error handling
    }

Builder methods:

.. code-block:: go

    resp := response.NewResponse(http.StatusCreated, user).
        WithTemplate("user.html").
        WithHeader("X-Request-ID", rid)

Format Resolution
-----------------

The response format is determined by a prioritized chain of resolver modules:

==========  ============================  ==========================================
Priority    Module                        Source
==========  ============================  ==========================================
10          ``format.route_override``      Route metadata: ``WithMeta("format", "json")``
20          ``format.query_param``         Query string: ``?format=json``
30          ``format.content_negotiate``   ``Accept`` header: ``Accept: text/html``
40          ``format.default``             Configured default format
==========  ============================  ==========================================

The first resolver that returns a format wins. Each resolver is an independent
module that can be disabled via config.

Using Format Resolution
~~~~~~~~~~~~~~~~~~~~~~~~

The most common pattern is to set format via route metadata:

.. code-block:: go

    // This route always returns JSON
    router.Get("/api/posts", listPosts,
        request.WithMeta("format", "json"),
    )

    // This route always returns HTML
    router.Get("/posts", listPosts,
        request.WithMeta("format", "html"),
    )

For routes that should respond to client preferences, omit the metadata and
let content negotiation or the default take effect.

Formatters
----------

A ``Formatter`` serializes response data into bytes:

.. code-block:: go

    type Formatter interface {
        Name() string
        ContentType() string
        Format(ctx context.Context, resp *Response) ([]byte, error)
    }

Built-in formatters:

JSON Formatter
~~~~~~~~~~~~~~

Serializes ``Data`` as JSON. Supports pretty-printing via ``?pretty=true``.

.. code-block:: go

    // Compact
    curl http://localhost:8080/api/posts
    // {"posts":[...]}

    // Pretty
    curl http://localhost:8080/api/posts?pretty=true
    // {
    //   "posts": [...]
    // }

HTML Formatter
~~~~~~~~~~~~~~

Renders ``Data`` through Go's ``html/template``. The ``Template`` field
selects which template file to use:

.. code-block:: go

    resp := response.NewResponse(http.StatusOK, map[string]any{
        "Title": "Hello",
        "Posts": posts,
    }).WithTemplate("posts.html")

Templates are loaded from the configured directory (default ``./templates``).
Templates are parsed lazily on first use and cached.

Custom Formatters
-----------------

Register custom formatters on the pipeline:

.. code-block:: go

    type XMLFormatter struct{}

    func (f *XMLFormatter) Name() string        { return "xml" }
    func (f *XMLFormatter) ContentType() string { return "application/xml" }
    func (f *XMLFormatter) Format(ctx context.Context, resp *response.Response) ([]byte, error) {
        return xml.Marshal(resp.Data)
    }

    // During module Init
    pipeline, _ := kernel.GetResource[*response.Pipeline](k, "response.pipeline")
    pipeline.RegisterFormatter(&XMLFormatter{})

Now ``?format=xml`` or ``Accept: application/xml`` will use your formatter.

Error Responses
---------------

Set the ``Error`` field to trigger error handling:

.. code-block:: go

    resp := &response.Response{
        Status: http.StatusNotFound,
        Error:  fmt.Errorf("post not found"),
    }
    return pipeline.Write(ctx.Context(), ctx.Writer, resp)

Error responses:

1. Fire the ``request.error`` hook
2. Serialize an error payload in the resolved format:
   ``{"error": "post not found", "status": 404}``
3. Fall back to plain text if serialization fails

Configuration
-------------

.. code-block:: toml

    [response]
    default_format = "json"        # fallback when no resolver matches
    template_dir = "./templates"   # directory for HTML templates

Disabling individual resolvers:

.. code-block:: toml

    [modules]
    "format.query_param" = false       # disable ?format= support
    "format.content_negotiate" = false  # disable Accept header negotiation
