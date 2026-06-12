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

For the complete list of framework-defined hook points, custom hook point
conventions, and the Fire/Resolve API, see :doc:`/reference/hooks`.

.. seealso::

   **Examples**

   - ``cmd/examples/kitchen-sink/modules/forms/module.go`` -- custom hook points (``form.before_submit``, ``form.after_submit``)
   - ``cmd/examples/kitchen-sink/main.go`` -- format resolution hooks wired through module registration

   **Source code**

   - Hook bus implementation (registration, fire, resolve, config-aware filtering): ``kernel/hook.go``
