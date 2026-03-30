Hook Bus
========

The hook bus is the kernel's mechanism for aspect-oriented programming. It
allows modules to register behavior at named lifecycle points without modifying
core code.

Core Properties
---------------

- **Priority-ordered** -- Hooks execute in numeric priority order (lower runs first)
- **Short-circuit capable** -- Resolution hooks can stop the chain early
- **Config-aware** -- Hooks from disabled modules are skipped per-request

Registering Hooks
-----------------

Standard hooks run at a named point and return an error:

.. code-block:: go

    func (m *MyModule) Init(k *kernel.Kernel) error {
        k.Hook("request.before", 10, m.onRequestBefore)
        return nil
    }

    func (m *MyModule) onRequestBefore(ctx context.Context) error {
        // runs before every request handler
        return nil
    }

When called during ``Init()``, hooks are automatically associated with the
initializing module. This association is used for config-aware execution.

Resolution Hooks
~~~~~~~~~~~~~~~~

Resolution hooks participate in a short-circuit chain. The first handler to
return ``resolved = true`` wins:

.. code-block:: go

    k.HookResolve("response.format.resolve", 20, func(ctx context.Context) (any, bool, error) {
        format := getFormatFromSomewhere(ctx)
        if format != "" {
            return format, true, nil   // resolved -- stop chain
        }
        return nil, false, nil         // not resolved -- try next
    })

Priority
--------

Priority is a numeric value. Lower numbers execute first:

.. code-block:: go

    k.Hook("request.before", 10, earlyHook)   // runs first
    k.Hook("request.before", 50, middleHook)   // runs second
    k.Hook("request.before", 90, lateHook)     // runs third

Convention for built-in modules:

==========  ==================================
Range       Usage
==========  ==================================
1-19        Framework internals, early setup
20-49       Application middleware
50-79       Application logic
80-99       Cleanup, late-stage processing
==========  ==================================

Config-Aware Execution
-----------------------

Before executing a hook, the bus checks whether its owning module is enabled
in the current request's config snapshot:

::

    For each hook at this point (sorted by priority):
        1. Look up owning module in config snapshot
        2. If module is disabled, skip
        3. Execute handler
        4. If error, stop chain
        5. If resolution hook resolved, stop chain

This means two concurrent requests can execute different hook chains if the
runtime config changed between their starts.

Hooks registered outside of module ``Init()`` (e.g., directly on the kernel)
are associated with the ``"kernel"`` module, which is always enabled.

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
