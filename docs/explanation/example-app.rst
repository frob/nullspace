About the Example Application
=============================

The repository includes two entry points that demonstrate how the framework's
concepts fit together in practice:

- ``cmd/nullspace/`` -- The installable binary with convention-based routing
- ``cmd/examples/kitchen-sink/`` -- A library-usage example with explicit route wiring

This page explores the design patterns and architectural decisions visible in
these examples.

Using the Nullspace Binary
--------------------------

The fastest way to see the framework in action::

    cd cmd/examples/kitchen-sink
    nullspace

Or scaffold a fresh project anywhere::

    mkdir mysite && cd mysite
    nullspace init
    nullspace

The binary auto-discovers content collections from ``content/`` subdirectories
and registers HTML + JSON routes for each.

Using the Library Example
--------------------------

The ``cmd/examples/kitchen-sink/`` directory shows how to use Nullspace as a Go library
with full control over module wiring and route definitions::

    cd cmd/examples/kitchen-sink
    go run .

Endpoints
---------

================================================  ===========  =====================
URL                                               Format       Description
================================================  ===========  =====================
``http://localhost:8080/``                        HTML         Home page
``http://localhost:8080/posts``                   HTML         Post list
``http://localhost:8080/posts/:id``               HTML         Single post
``http://localhost:8080/api/posts``               JSON         Post list (API)
``http://localhost:8080/api/posts/:id``           JSON         Single post (API)
``http://localhost:8080/api/posts?pretty=true``   JSON         Pretty-printed
``http://localhost:8080/api/health``              JSON         Health check
``http://localhost:8080/chat``                    HTML         Chat room UI
``ws://localhost:8080/ws/chat``                   WebSocket    Chat endpoint
``http://localhost:8080/css/style.css``           CSS          Static file
================================================  ===========  =====================

Project Structure
-----------------

::

    cmd/examples/kitchen-sink/
    ├── main.go              Application entry point (library usage)
    ├── nullspace.toml       Configuration
    ├── content/
    │   └── posts/
    │       ├── hello-world.md
    │       └── architecture.md
    ├── templates/
    │   ├── home.html
    │   ├── posts.html
    │   ├── post.html
    │   └── chat.html
    └── public/
        └── css/
            └── style.css

What It Demonstrates
--------------------

**Module wiring** -- The ``main.go`` registers all modules in dependency order:
logging, request adapter, response pipeline, format resolvers, static files,
and file data.

**Format resolution** -- HTML routes use ``WithMeta("format", "html")`` to
force HTML rendering. API routes use ``WithMeta("format", "json")``. The same
data can be served in either format.

**File-based content** -- Posts are stored as markdown files with YAML
frontmatter. The file data module parses them into entities with structured
metadata and body content.

**Static file fallback** -- CSS is served from ``public/`` as a fallback when
no dynamic route matches.

**Per-request logging** -- Every request gets a unique ID and structured
logging with method, path, status, and duration.

**Graceful shutdown** -- The app listens for SIGINT/SIGTERM and stops all
modules in reverse order.

Binary vs Library
-----------------

==========================  ==============================  ==============================
Feature                     Binary (``nullspace``)          Library (``go run .``)
==========================  ==============================  ==============================
Route definition            Convention-based                Explicit in code
Custom middleware            Not yet supported               Full control
Custom modules              Not yet supported               Full control
Configuration               ``nullspace.toml`` + env        ``nullspace.toml`` + env
Content management          File-based, auto-discovered     File-based or any source
Best for                    Content sites, prototyping      APIs, custom applications
==========================  ==============================  ==============================

.. seealso::

   **Source code**

   - Kitchen-sink entry point (module wiring, custom handlers): ``cmd/examples/kitchen-sink/main.go``
   - Kitchen-sink configuration: ``cmd/examples/kitchen-sink/nullspace.toml``
   - Auth module: ``cmd/examples/kitchen-sink/modules/auth/module.go``
   - Forms module: ``cmd/examples/kitchen-sink/modules/forms/module.go``
   - Chat module: ``cmd/examples/kitchen-sink/modules/chat/module.go``
   - Content files (markdown with YAML frontmatter): ``cmd/examples/kitchen-sink/content/posts/``
   - HTML templates: ``cmd/examples/kitchen-sink/templates/``

   See :doc:`/explanation/example-modules` for a detailed walkthrough of the auth, forms, and chat modules.
