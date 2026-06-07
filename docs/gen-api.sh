#!/usr/bin/env bash
#
# Generate API documentation under docs/content/api/ from Go source.
#
# This script runs `gomarkdoc` on each package and wraps the output with
# Hugo frontmatter so the resulting markdown drops into the Hugo Book theme.
# It is invoked by the docs Dockerfile, but can also run on the host:
#
#     go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest
#     ./docs/gen-api.sh
#
# Edit doc comments in the source — never edit content/api/*.md by hand.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="$ROOT/docs/content/api"

mkdir -p "$OUT_DIR"

# format: package_dir | output_filename | display_title | weight
PKGS=(
  "./kernel|kernel.md|kernel|1"
  "./core/transport|core-transport.md|core/transport|2"
  "./core/nslog|core-nslog.md|core/nslog|3"
  "./core/request|core-request.md|core/request|4"
  "./core/response|core-response.md|core/response|5"
  "./core/routing|core-routing.md|core/routing|6"
  "./core/tcp|core-tcp.md|core/tcp|7"
  "./core/ipc|core-ipc.md|core/ipc|8"
  "./module/data|module-data.md|module/data|9"
  "./module/data/file|module-data-file.md|module/data/file|10"
  "./module/data/sql|module-data-sql.md|module/data/sql|11"
  "./module/data/static|module-data-static.md|module/data/static|12"
  "./module/data/bridge|module-data-bridge.md|module/data/bridge|13"
  "./module/session|module-session.md|module/session|14"
  "./module/httpsecurity|module-httpsecurity.md|module/httpsecurity|15"
  "./module/websocket|module-websocket.md|module/websocket|16"
)

cd "$ROOT"

for entry in "${PKGS[@]}"; do
  IFS='|' read -r pkg file title weight <<<"$entry"
  out="$OUT_DIR/$file"

  # Skip packages with no Go files (e.g. empty parent dirs).
  if ! ls "$pkg"/*.go >/dev/null 2>&1; then
    echo "skip $pkg (no .go files)"
    continue
  fi

  {
    printf -- '---\n'
    printf -- 'title: %s\n' "$title"
    printf -- 'weight: %s\n' "$weight"
    printf -- 'bookHidden: false\n'
    printf -- '---\n\n'
    printf -- '<!-- Generated from `%s` by gomarkdoc. Edit doc comments in source. -->\n\n' "$pkg"
    gomarkdoc --repository.url 'https://github.com/frob/nullspace' \
              --repository.default-branch '0.0.x' \
              --repository.path '/' \
              "$pkg"
  } > "$out"
  echo "generated $out"
done
