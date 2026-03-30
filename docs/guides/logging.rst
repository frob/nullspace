Logging
=======

Nullspace provides structured, leveled logging through a ``Logger`` port with
a default ``log/slog`` adapter.

Logger Port
-----------

The ``Logger`` interface mirrors ``slog`` semantics:

.. code-block:: go

    type Logger interface {
        Debug(msg string, args ...any)
        Info(msg string, args ...any)
        Warn(msg string, args ...any)
        Error(msg string, args ...any)
        With(args ...any) Logger
    }

Arguments are key-value pairs: ``logger.Info("request", "method", "GET", "path", "/api")``.

``With()`` returns a child logger with fields permanently attached.

Configuration
-------------

.. code-block:: toml

    [log]
    level = "info"    # debug, info, warn, error
    format = "text"   # text, json

Environment override::

    NULLSPACE_LOG_LEVEL=debug
    NULLSPACE_LOG_FORMAT=json

Two Logger Scopes
-----------------

Kernel Logger
~~~~~~~~~~~~~

Created at kernel construction. Used for lifecycle events:

.. code-block:: go

    k.Logger().Info("custom event", "key", "value")

Output:

.. code-block:: text

    time=2025-01-15T10:00:00 level=INFO msg="custom event" key=value

Per-Request Logger
~~~~~~~~~~~~~~~~~~

Created by the request adapter for each request. Enriched with:

- **request_id** -- Unique hex ID for tracing
- **method** -- HTTP method
- **path** -- Request URL path

.. code-block:: go

    func myHandler(ctx *request.Context) error {
        ctx.Logger().Info("processing")
        // time=... level=INFO msg=processing request_id=a1b2c3 method=GET path=/api/posts
        return nil
    }

Automatic Logging
-----------------

The logging module registers hooks that automatically log:

==========================  =======  ==================================
Event                       Level    Fields
==========================  =======  ==================================
Request received            INFO     request_id, method, path
Request complete            INFO     request_id, method, path, status, duration_ms
Module init                 INFO     module name
Module start                INFO     module name
Module stop                 INFO     module name
Hook registration           DEBUG    hook point, module, priority
Resource provided           DEBUG    resource key
Route matched               DEBUG    pattern
Static file served          DEBUG    file path
Handler error               ERROR    error message
==========================  =======  ==================================

Swapping the Logger
-------------------

Replace the default slog adapter with any implementation of ``Logger``:

.. code-block:: go

    type zerologAdapter struct {
        logger zerolog.Logger
    }

    func (z *zerologAdapter) Info(msg string, args ...any) {
        event := z.logger.Info()
        for i := 0; i < len(args)-1; i += 2 {
            event = event.Interface(fmt.Sprint(args[i]), args[i+1])
        }
        event.Msg(msg)
    }

    // ... implement Debug, Warn, Error, With

    k := kernel.New(
        kernel.WithLogger(&zerologAdapter{logger: zerolog.New(os.Stderr)}),
    )

All framework internals use the ``Logger`` port, so the adapter is fully
swappable.

Accessing the Logger
--------------------

In handlers:

.. code-block:: go

    ctx.Logger().Info("handler message")

In modules (via kernel):

.. code-block:: go

    k.Logger().Info("module message")

From context (in hooks):

.. code-block:: go

    func myHook(ctx context.Context) error {
        logger := nslog.FromContext(ctx)
        if logger != nil {
            logger.Info("hook message")
        }
        return nil
    }

From the service locator:

.. code-block:: go

    logger, _ := kernel.GetResource[kernel.Logger](k, "logger")
