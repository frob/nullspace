Nullspace Framework Documentation
===================================

Nullspace is a multi-transport application framework written in Go for serving
APIs, HTML, WebSockets, and more. It uses hexagonal architecture to separate
concerns and aspect-oriented programming via a hook bus and functional
middleware.

This documentation follows the `Diátaxis <https://diataxis.fr/>`_ framework
and is organized into four sections:

- **Tutorials** take you through a series of steps to build your first
  application. Start here if you are new to Nullspace.

- **How-to guides** provide directions for solving specific problems and
  accomplishing common tasks. They assume you already have a working Nullspace
  application.

- **Reference** contains the technical description of every package, interface,
  type, and function in the framework.

- **Explanation** discusses the design decisions, architecture, and concepts
  behind the framework to deepen your understanding.

.. toctree::
   :maxdepth: 2
   :caption: Tutorials

   tutorials/quickstart

.. toctree::
   :maxdepth: 2
   :caption: How-to Guides

   how-to/installation
   how-to/routing
   how-to/responses
   how-to/logging
   how-to/data
   how-to/custom-modules
   how-to/session
   how-to/httpsecurity
   how-to/websocket
   how-to/oidc
   how-to/deployment

.. toctree::
   :maxdepth: 2
   :caption: Reference

   reference/kernel
   reference/request
   reference/response
   reference/routing
   reference/hooks
   reference/nslog
   reference/session
   reference/httpsecurity
   reference/websocket
   reference/oidc
   reference/data

.. toctree::
   :maxdepth: 2
   :caption: Explanation

   explanation/architecture
   explanation/modules
   explanation/hooks
   explanation/configuration
   explanation/middleware
   explanation/example-app
   explanation/example-modules
