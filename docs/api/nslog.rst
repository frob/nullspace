nslog
=====

``import "github.com/frob/nullspace/nslog"``

The nslog package provides the logging module and context helpers for
per-request logger propagation.

Types
-----

Module
~~~~~~

.. code-block:: go

    func New() *Module

Creates a new logging module. Implements ``Module`` and ``Configurable``.

Reconfigures the kernel logger during Init based on config. Registers hooks
at ``request.received`` (priority 10) and ``request.complete`` (priority 90).

**Config struct:**

.. code-block:: go

    type Config struct {
        Level  string `json:"level"`   // debug, info, warn, error
        Format string `json:"format"`  // text, json
    }

Context Helpers
---------------

.. code-block:: go

    // Logger
    func FromContext(ctx context.Context) kernel.Logger
    func WithLogger(ctx context.Context, l kernel.Logger) context.Context

    // Request info
    func WithRequestID(ctx context.Context, id string) context.Context
    func WithRequestInfo(ctx context.Context, method, path string) context.Context
    func WithResponseStatus(ctx context.Context, status int) context.Context

``WithRequestInfo`` also sets the request start time (``time.Now()``).

These are primarily used by the request adapter to set up the per-request
context. Application code typically accesses the logger via
``ctx.Logger()`` on the request context.
