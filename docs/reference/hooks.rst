Hook Reference
==============

For an explanation of how the hook bus works, registration, priorities, and
config-aware execution, see :doc:`/explanation/hooks`.

Hook Points
-----------

Framework-defined hook points:

Kernel Lifecycle
~~~~~~~~~~~~~~~~

==============================  ==========================================
Hook Point                      When
==============================  ==========================================
``kernel.before_init``          Before any module Init
``kernel.after_init``           After all modules Init
``kernel.before_start``         Before any module Start
``kernel.after_start``          After all modules Start
``kernel.before_stop``          Before any module Stop
``kernel.after_stop``           After all modules Stop
==============================  ==========================================

Request Lifecycle
~~~~~~~~~~~~~~~~~

==============================  ==========================================
Hook Point                      When
==============================  ==========================================
``request.received``            Raw request received, before routing
``request.routed``              Route matched or fallback determined
``request.before``              Before handler execution
``request.after``               After handler execution
``request.error``               Error occurred during handling
``request.complete``            Response sent, cleanup phase
==============================  ==========================================

Response Lifecycle
~~~~~~~~~~~~~~~~~~

==============================  ==========================================
Hook Point                      When
==============================  ==========================================
``response.format.resolve``     Determine format (resolution, short-circuit)
``response.before_write``       Before serialization
``response.after_write``        After serialization
==============================  ==========================================

Data Lifecycle
~~~~~~~~~~~~~~

==============================  ==========================================
Hook Point                      When
==============================  ==========================================
``data.before_read``            Before data read (policy checks)
``data.after_read``             After data read
``data.before_write``           Before data write
``data.after_write``            After data write
==============================  ==========================================

.. note::

   The SQL module registers a ``kernel.after_init`` hook (priority 10) that
   runs all pending database migrations. Modules register their migrations
   during ``Init`` via the ``"data.sql.migrations"`` service locator key.
   See :doc:`/how-to/data` for details.

WebSocket Lifecycle
~~~~~~~~~~~~~~~~~~~

==============================  ==========================================
Hook Point                      When
==============================  ==========================================
``websocket.connected``         Connection established after upgrade
``websocket.message``           Message received (before handler)
``websocket.disconnected``      Connection closed
``websocket.error``             Non-clean close error
==============================  ==========================================

Custom Hook Points
------------------

Modules can define their own hook points. The naming convention is
``component.event``:

.. code-block:: go

    // Define a custom hook point in your module
    func (m *MyModule) doSomething(ctx context.Context) error {
        if err := m.kernel.Fire("mymodule.before_action", ctx); err != nil {
            return err
        }
        // ... do the action ...
        return m.kernel.Fire("mymodule.after_action", ctx)
    }

Other modules can hook into it:

.. code-block:: go

    func (m *AuditModule) Init(k *kernel.Kernel) error {
        k.Hook("mymodule.before_action", 10, m.auditAction)
        return nil
    }

Firing Hooks
------------

The kernel provides convenience methods:

.. code-block:: go

    // Fire all hooks at a point
    err := k.Fire("request.before", ctx)

    // Resolve: fire resolution hooks, return first resolved value
    val, err := k.Resolve("response.format.resolve", ctx)

The ``HookBus`` is also accessible directly via ``k.HookBus()`` for advanced
use cases.
