Quickstart
==========

There are two ways to use Nullspace: as an installable binary (convention-based,
like Hugo) or as a Go library (full control).

Using the Binary
----------------

Scaffold a new project::

    mkdir mysite && cd mysite
    nullspace init

This creates:

.. code-block:: text

    mysite/
    ├── nullspace.toml
    ├── content/
    │   └── posts/
    │       └── hello-world.md
    ├── templates/
    │   ├── home.html
    │   ├── posts.html
    │   └── post.html
    └── public/
        └── css/
            └── style.css

Start the server::

    nullspace

Visit http://localhost:8080. Content collections are auto-discovered from
``content/`` subdirectories. Each collection gets both HTML and JSON routes:

- ``/posts`` and ``/posts/:id`` -- HTML (rendered with templates)
- ``/api/posts`` and ``/api/posts/:id`` -- JSON
- ``/api/health`` -- Health check

Add a new collection by creating a subdirectory in ``content/``::

    mkdir content/pages
    cat > content/pages/about.md << 'EOF'
    ---
    title: About
    ---

    This is the about page.
    EOF

Restart ``nullspace`` and visit ``/pages/about`` (HTML) or ``/api/pages/about``
(JSON).

Using the Library
-----------------

For full control over routing, modules, and middleware, use Nullspace as a
Go library::

    mkdir myapp && cd myapp
    go mod init myapp
    go get github.com/frob/nullspace

Create ``main.go``:

.. code-block:: go

    package main

    import (
        "context"
        "net/http"
        "os"
        "os/signal"
        "syscall"

        "github.com/frob/nullspace/kernel"
        "github.com/frob/nullspace/nslog"
        "github.com/frob/nullspace/request"
        "github.com/frob/nullspace/response"
    )

    func main() {
        k := kernel.New()

        adapter := request.NewAdapter()
        pipeline := response.NewPipeline()

        k.Use(nslog.New())
        k.Use(adapter)
        k.Use(pipeline)
        k.Use(response.NewFormatDefault())

        ctx := context.Background()
        k.Init(ctx)

        adapter.Router().Get("/hello", func(ctx *request.Context) error {
            resp := response.NewResponse(http.StatusOK, map[string]string{
                "message": "hello world",
            })
            return pipeline.Write(ctx.Context(), ctx.Writer, resp)
        })

        k.Start(ctx)

        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
        <-quit
        k.Stop(ctx)
    }

Run it::

    go run .

Test it::

    curl http://localhost:8080/hello
    # {"message":"hello world"}

Adding Configuration
--------------------

Create ``nullspace.toml`` in the same directory:

.. code-block:: toml

    [request]
    addr = ":3000"

    [log]
    level = "debug"
    format = "text"

    [response]
    default_format = "json"

Now the server listens on port 3000 with debug logging.

Environment variables override TOML values::

    NULLSPACE_REQUEST_ADDR=:9090 nullspace

Adding Middleware
-----------------

Middleware uses the standard functional pattern:

.. code-block:: go

    func cors(next request.HandlerFunc) request.HandlerFunc {
        return func(ctx *request.Context) error {
            ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
            return next(ctx)
        }
    }

    // Register globally
    adapter.Use(cors)

Adding Static Files
-------------------

Create a ``public/`` directory with your static files, then register the
static module:

.. code-block:: go

    import "github.com/frob/nullspace/data/static"

    k.Use(static.New())

Files in ``./public`` are served as a fallback when no dynamic route matches.
A request to ``/css/style.css`` serves ``public/css/style.css``.

.. note::

   Dynamic routes always take precedence over static files. If you register
   a route at ``/css/style.css``, the route handler runs instead of serving
   the file.

Next Steps
----------

- :doc:`example-app` -- Walk through the full example application
- :doc:`../concepts/architecture` -- Understand the hexagonal architecture
- :doc:`../guides/custom-modules` -- Write your own modules
