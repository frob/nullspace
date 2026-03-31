Nullspace Framework Documentation
===================================

Nullspace is an HTTP application framework written in Go for serving APIs, HTML,
WebSockets, LLM interfaces, and future modalities. It uses hexagonal architecture
to separate concerns and aspect-oriented programming via a hook bus and functional
middleware.

.. toctree::
   :maxdepth: 2
   :caption: Getting Started

   getting-started/installation
   getting-started/quickstart
   getting-started/example-app

.. toctree::
   :maxdepth: 2
   :caption: Core Concepts

   concepts/architecture
   concepts/modules
   concepts/hooks
   concepts/configuration
   concepts/middleware

.. toctree::
   :maxdepth: 2
   :caption: Guides

   guides/routing
   guides/responses
   guides/logging
   guides/data
   guides/custom-modules
   guides/session
   guides/httpsecurity
   guides/example-modules
   guides/deployment

.. toctree::
   :maxdepth: 2
   :caption: API Reference

   api/kernel
   api/request
   api/response
   api/routing
   api/nslog
   api/session
   api/httpsecurity
   api/data
