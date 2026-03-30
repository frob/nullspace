FROM python:3.12-slim

RUN pip install --no-cache-dir sphinx sphinx-rtd-theme

WORKDIR /docs

ENTRYPOINT ["sphinx-build", "-b", "html", ".", "_build/html"]
