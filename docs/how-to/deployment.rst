Deployment
==========

Nullspace compiles to a single static binary with no runtime dependencies.
Deploy by copying the binary and your project directory to the target host.

Binary Deployment
-----------------

The simplest deployment: copy the binary and project files.

Build a release binary::

    task dist

Or cross-compile for a specific target::

    GOOS=linux GOARCH=amd64 task dist

Copy to the server::

    scp bin/nullspace myserver:/usr/local/bin/
    scp -r myproject/ myserver:/opt/myapp/

Run on the server::

    cd /opt/myapp
    nullspace

Package Manager Installation
-----------------------------

Pre-built packages are published with each release:

**Debian / Ubuntu:**

.. code-block:: bash

    sudo dpkg -i nullspace_<version>_linux_amd64.deb
    # Binary installs to /usr/bin/nullspace
    # Default config installs to /etc/nullspace/nullspace.toml

**Rocky / RHEL / Fedora:**

.. code-block:: bash

    sudo rpm -i nullspace_<version>_linux_amd64.rpm

**Arch Linux:**

.. code-block:: bash

    sudo pacman -U nullspace_<version>_linux_amd64.pkg.tar.zst

Both ``amd64`` and ``arm64`` packages are available.

Systemd Service
~~~~~~~~~~~~~~~

Create ``/etc/systemd/system/nullspace.service``:

.. code-block:: ini

    [Unit]
    Description=Nullspace
    After=network.target

    [Service]
    Type=simple
    User=nullspace
    WorkingDirectory=/opt/myapp
    ExecStart=/usr/bin/nullspace
    Restart=on-failure
    RestartSec=5

    # Environment overrides
    Environment=NULLSPACE_LOG_LEVEL=info
    Environment=NULLSPACE_LOG_FORMAT=json
    Environment=NULLSPACE_REQUEST_ADDR=:8080

    [Install]
    WantedBy=multi-user.target

Enable and start::

    sudo systemctl enable nullspace
    sudo systemctl start nullspace
    sudo journalctl -u nullspace -f

Docker Deployment
-----------------

For container-based deployments, mount your project directory into the
container:

.. code-block:: bash

    docker run -d \
      --name nullspace \
      -p 8080:8080 \
      -v /opt/myapp:/app \
      ghcr.io/frob/nullspace

Multi-arch images (``amd64`` and ``arm64``) are published to
``ghcr.io/frob/nullspace``.

Docker Compose
~~~~~~~~~~~~~~

.. code-block:: yaml

    services:
      nullspace:
        image: ghcr.io/frob/nullspace:latest
        ports:
          - "8080:8080"
        volumes:
          - ./myproject:/app
        environment:
          - NULLSPACE_LOG_FORMAT=json

Building a Custom Image
~~~~~~~~~~~~~~~~~~~~~~~

If your application uses the library approach (custom ``main.go``), use
the project Dockerfile as a starting point:

.. code-block:: dockerfile

    FROM golang:1.25-alpine AS builder
    WORKDIR /src
    COPY go.mod go.sum ./
    RUN go mod download
    COPY . .
    RUN CGO_ENABLED=0 go build -o /bin/myapp .

    FROM alpine:3.21
    RUN apk add --no-cache ca-certificates
    COPY --from=builder /bin/myapp /usr/local/bin/myapp
    COPY templates /app/templates
    COPY public /app/public
    COPY content /app/content
    COPY nullspace.toml /app/nullspace.toml
    WORKDIR /app
    EXPOSE 8080
    ENTRYPOINT ["myapp"]

Configuration for Production
-----------------------------

Use environment variables for secrets and per-environment settings. Keep
``nullspace.toml`` for defaults that are safe to commit:

.. code-block:: toml

    # nullspace.toml -- committed to repo
    [request]
    addr = ":8080"

    [log]
    level = "info"
    format = "json"

    [data.sql]
    driver = "postgres"

.. code-block:: bash

    # Environment -- set per deployment
    export NULLSPACE_DATA_SQL_DSN="postgres://user:pass@db:5432/myapp"
    export NULLSPACE_LOG_LEVEL="warn"

Reverse Proxy
~~~~~~~~~~~~~

Nullspace serves HTTP directly but should sit behind a reverse proxy in
production for TLS termination and load balancing.

**Nginx:**

.. code-block:: nginx

    server {
        listen 443 ssl;
        server_name myapp.example.com;

        ssl_certificate     /etc/ssl/certs/myapp.pem;
        ssl_certificate_key /etc/ssl/private/myapp.key;

        location / {
            proxy_pass http://127.0.0.1:8080;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }

**Caddy** (automatic TLS):

.. code-block:: text

    myapp.example.com {
        reverse_proxy localhost:8080
    }

Health Checks
~~~~~~~~~~~~~

The ``/api/health`` endpoint returns ``{"status":"ok"}`` and can be used
by load balancers, container orchestrators, and monitoring systems.

Releasing
---------

Releases are managed with `GoReleaser <https://goreleaser.com>`_. A single
command builds all artifacts:

.. code-block:: bash

    # Validate config
    task release:check

    # Test the full pipeline locally
    task release:snapshot

    # Tag and publish
    git tag v1.0.0
    task release

This produces:

- Tarballs for macOS (amd64, arm64) and Linux (amd64, arm64)
- ``.deb`` packages for Debian/Ubuntu
- ``.rpm`` packages for Rocky/RHEL/Fedora
- ``.pkg.tar.zst`` packages for Arch Linux
- Homebrew cask formula
- Multi-arch Docker images on ``ghcr.io``
- SHA-256 checksums
