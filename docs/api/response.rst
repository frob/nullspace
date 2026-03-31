response
========

``import "github.com/frob/nullspace/core/response"``

The response package provides format resolution, formatters, and the response
pipeline.

Types
-----

Response
~~~~~~~~

.. code-block:: go

    type Response struct {
        Status   int
        Headers  map[string]string
        Data     any
        Template string
        Error    error
    }

    func NewResponse(status int, data any) *Response
    func (r *Response) WithTemplate(name string) *Response
    func (r *Response) WithHeader(key, value string) *Response

Formatter
~~~~~~~~~

.. code-block:: go

    type Formatter interface {
        Name() string
        ContentType() string
        Format(ctx context.Context, resp *Response) ([]byte, error)
    }

Pipeline
~~~~~~~~

.. code-block:: go

    func NewPipeline() *Pipeline

Creates a new response pipeline module. Implements ``Module`` and
``Configurable``.

.. code-block:: go

    func (p *Pipeline) RegisterFormatter(f Formatter)
    func (p *Pipeline) Write(ctx context.Context, w http.ResponseWriter, resp *Response) error

**Config struct:**

.. code-block:: go

    type PipelineConfig struct {
        DefaultFormat string `json:"default_format"`  // default "json"
        TemplateDir   string `json:"template_dir"`    // default "./templates"
    }

Built-in Formatters
~~~~~~~~~~~~~~~~~~~

.. code-block:: go

    // JSON: serializes Data as JSON. Supports ?pretty=true.
    type JSONFormatter struct{}

    // HTML: renders Data through Go html/template.
    func NewHTMLFormatter(dir string) *HTMLFormatter

Format Resolution Modules
~~~~~~~~~~~~~~~~~~~~~~~~~

Each is a ``Module`` with a ``Config()`` method:

.. code-block:: go

    func NewFormatRouteOverride() *FormatRouteOverride     // priority 10
    func NewFormatQueryParam() *FormatQueryParam           // priority 20
    func NewFormatContentNegotiate() *FormatContentNegotiate // priority 30
    func NewFormatDefault() *FormatDefault                 // priority 40

Context Helpers
~~~~~~~~~~~~~~~

Set by the request adapter for format resolvers to read:

.. code-block:: go

    func WithHTTPRequest(ctx context.Context, r *http.Request) context.Context
    func HTTPRequestFromContext(ctx context.Context) *http.Request
    func WithRouteFormat(ctx context.Context, format string) context.Context
    func WithQueryFormat(ctx context.Context, format string) context.Context
    func WithAcceptHeader(ctx context.Context, accept string) context.Context
