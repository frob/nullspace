request
=======

``import "github.com/frob/nullspace/core/request"``

The request package provides the HTTP adapter, router, middleware chain, and
request context.

Types
-----

HandlerFunc
~~~~~~~~~~~

.. code-block:: go

    type HandlerFunc func(ctx *Context) error

Framework handler signature. Return a non-nil error to trigger error handling.

Middleware
~~~~~~~~~~

.. code-block:: go

    type Middleware func(HandlerFunc) HandlerFunc

Wraps a handler with before/after behavior.

Context
~~~~~~~

.. code-block:: go

    type Context struct {
        Request *http.Request
        Writer  http.ResponseWriter
        // private fields
    }

    func (c *Context) Context() context.Context
    func (c *Context) Logger() kernel.Logger
    func (c *Context) Snapshot() *kernel.Snapshot
    func (c *Context) Param(name string) string
    func (c *Context) Route() *RouteMatch
    func (c *Context) State(key string) (any, bool)
    func (c *Context) SetState(key string, val any)

``Request`` and ``Writer`` are the standard ``net/http`` types. All other
fields are accessed through methods.

Adapter
~~~~~~~

.. code-block:: go

    func NewAdapter() *Adapter

Creates a new HTTP adapter module. Implements ``Module`` and ``Configurable``.

.. code-block:: go

    func (a *Adapter) Use(mw ...Middleware)
    func (a *Adapter) Router() *Router
    func (a *Adapter) Fallback(handler HandlerFunc)
    func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request)

**Config struct:**

.. code-block:: go

    type AdapterConfig struct {
        Addr string `json:"addr"`  // default ":8080"
    }

Router
~~~~~~

.. code-block:: go

    func NewRouter() *Router

    func (r *Router) Handle(method, pattern string, handler HandlerFunc, opts ...RouteOption)
    func (r *Router) Get(pattern string, handler HandlerFunc, opts ...RouteOption)
    func (r *Router) Post(pattern string, handler HandlerFunc, opts ...RouteOption)
    func (r *Router) Put(pattern string, handler HandlerFunc, opts ...RouteOption)
    func (r *Router) Delete(pattern string, handler HandlerFunc, opts ...RouteOption)
    func (r *Router) Patch(pattern string, handler HandlerFunc, opts ...RouteOption)
    func (r *Router) Match(method, path string) *RouteMatch

RouteMatch
~~~~~~~~~~

.. code-block:: go

    type RouteMatch struct {
        Pattern string
        Params  map[string]string
        Meta    map[string]string
    }

RouteOption
~~~~~~~~~~~

.. code-block:: go

    func WithMeta(key, value string) RouteOption
    func WithRouteMiddleware(mw ...Middleware) RouteOption

Variables
---------

.. code-block:: go

    var ErrNotHandled = fmt.Errorf("not handled")

Returned by fallback handlers to indicate the request was not handled.
