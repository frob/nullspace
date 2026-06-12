routing
=======

``import "github.com/frob/nullspace/core/routing"``

The routing package provides declarative TOML-based route configuration
with a handler/middleware registry, built-in handlers, and collections.

Types
-----

Module
~~~~~~

.. code-block:: go

    func New() *Module

Creates a new routing module. Implements ``Module`` and ``Configurable``.

.. code-block:: go

    func (m *Module) AddRoute(r Route)
    func (m *Module) AddCollection(c Collection)
    func (m *Module) LoadRoutes(data []byte) error
    func (m *Module) Table() *RouteTable
    func (m *Module) Registry() *Registry

``AddRoute`` and ``AddCollection`` add routes programmatically during
module Init. Duplicates are deduplicated by effective path + handler.

``LoadRoutes`` parses embedded TOML route data and merges it into the
config. Used by modules with their own ``routes.toml``.

``Table`` returns the route table after routes are resolved.

Registry
~~~~~~~~

.. code-block:: go

    func NewRegistry() *Registry

    func (r *Registry) HandleFunc(name string, h request.HandlerFunc)
    func (r *Registry) Middleware(name string, m request.Middleware)
    func (r *Registry) LookupHandler(name string) (request.HandlerFunc, error)
    func (r *Registry) LookupMiddleware(name string) (request.Middleware, error)
    func (r *Registry) ResolveMiddleware(names []string) ([]request.Middleware, error)
    func (r *Registry) HandlerNames() []string
    func (r *Registry) MiddlewareNames() []string

The registry is provided as ``"routing.registry"`` in the service locator.

Config Types
~~~~~~~~~~~~

.. code-block:: go

    type Config struct {
        Groups      map[string]Group `json:"groups" toml:"groups"`
        Routes      []Route          `json:"routes" toml:"routes"`
        Collections []Collection     `json:"collections" toml:"collections"`
    }

    type Group struct {
        Prefix     string   `json:"prefix" toml:"prefix"`
        Format     string   `json:"format" toml:"format"`
        Middleware []string `json:"middleware" toml:"middleware"`
    }

    type Route struct {
        Group      string   `json:"group" toml:"group"`
        Path       string   `json:"path" toml:"path"`
        Methods    []string `json:"methods" toml:"methods"`
        Handler    string   `json:"handler" toml:"handler"`
        Format     string   `json:"format" toml:"format"`
        Template   string   `json:"template" toml:"template"`
        Middleware []string `json:"middleware" toml:"middleware"`
        Collection string   `json:"collection" toml:"collection"`
        DataParam  string   `json:"data_param" toml:"data_param"`
        Redirect   string   `json:"redirect" toml:"redirect"`
        StatusCode int      `json:"status_code" toml:"status_code"`
    }

    type Collection struct {
        Name            string   `json:"name" toml:"name"`
        Source          string   `json:"source" toml:"source"`
        APIPrefix       string   `json:"api_prefix" toml:"api_prefix"`
        HTMLPrefix      string   `json:"html_prefix" toml:"html_prefix"`
        ListTemplate    string   `json:"list_template" toml:"list_template"`
        ItemTemplate    string   `json:"item_template" toml:"item_template"`
        ReadMiddleware  []string `json:"read_middleware" toml:"read_middleware"`
        WriteMiddleware []string `json:"write_middleware" toml:"write_middleware"`
    }

RouteTable
~~~~~~~~~~

.. code-block:: go

    type RouteTable struct {
        Entries []TableEntry
    }

    type TableEntry struct {
        Method     string
        Path       string
        Handler    string
        Format     string
        Middleware []string
        Template   string
    }

    func (t *RouteTable) Add(entry TableEntry)
    func (t *RouteTable) Print(w io.Writer)
    func (t *RouteTable) String() string

Service Locator Keys
~~~~~~~~~~~~~~~~~~~~

============================  ========================
Key                           Type
============================  ========================
``"routing"``                 ``*routing.Module``
``"routing.registry"``        ``*routing.Registry``
============================  ========================

Built-in Handler Names
~~~~~~~~~~~~~~~~~~~~~~

==============================  ==============================
Name                            Description
==============================  ==============================
``data.list``                   List entities from collection
``data.get``                    Get entity by ID param
``data.create``                 Create from request body
``data.update``                 Update from request body
``data.delete``                 Delete by ID param
``template``                    Render template only
``redirect``                    HTTP redirect
==============================  ==============================
