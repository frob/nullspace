Installation
============

Pre-built Binaries
------------------

Download from the `GitHub releases page <https://github.com/frob/nullspace/releases>`_
or use a package manager:

macOS (Homebrew)
~~~~~~~~~~~~~~~~

::

    brew install frob/tap/nullspace

Debian / Ubuntu
~~~~~~~~~~~~~~~~

::

    curl -LO https://github.com/frob/nullspace/releases/latest/download/nullspace_<version>_linux_amd64.deb
    sudo dpkg -i nullspace_*_linux_amd64.deb

Rocky / RHEL / Fedora
~~~~~~~~~~~~~~~~~~~~~~

::

    curl -LO https://github.com/frob/nullspace/releases/latest/download/nullspace_<version>_linux_amd64.rpm
    sudo rpm -i nullspace_*_linux_amd64.rpm

Arch Linux
~~~~~~~~~~

::

    curl -LO https://github.com/frob/nullspace/releases/latest/download/nullspace_<version>_linux_amd64.pkg.tar.zst
    sudo pacman -U nullspace_*_linux_amd64.pkg.tar.zst

Both ``amd64`` and ``arm64`` packages are available for all Linux distributions.

From Source
-----------

Requires Go 1.22 or later::

    go install github.com/frob/nullspace/cmd/nullspace@latest

Docker
------

::

    docker run --rm -v $(pwd):/app -p 8080:8080 ghcr.io/frob/nullspace

Mount your project directory to ``/app`` in the container.

Verify Installation
-------------------

::

    nullspace version

Use as a Library
-----------------

To use Nullspace as a Go library in your own application::

    go get github.com/frob/nullspace

See the :doc:`/tutorials/quickstart` tutorial for both usage patterns.

Dependencies
------------

Nullspace has minimal external dependencies:

- ``github.com/pelletier/go-toml/v2`` -- TOML configuration parsing
- ``gopkg.in/yaml.v3`` -- YAML frontmatter parsing (file data module)
- ``modernc.org/sqlite`` -- SQLite driver, pure Go, no CGO (SQL data module)

The binary is statically compiled with no runtime dependencies.
