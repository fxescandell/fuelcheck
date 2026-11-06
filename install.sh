#!/bin/sh
set -eu

# Instala la CLI desde el fork público, sin necesitar una release publicada.
# Uso: sh install.sh [rama-o-etiqueta]
repo="https://github.com/fxescandell/fuelcheck.git"
ref="${1:-main}"

if [ "$#" -gt 1 ]; then
    printf 'Uso: %s [rama-o-etiqueta]\n' "$0" >&2
    exit 2
fi

for tool in git go; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        printf 'Error: se necesita %s para instalar ai-usage.\n' "$tool" >&2
        exit 1
    fi
done

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/ai-usage-install.XXXXXX")
trap 'rm -rf "$tmp_dir"' 0

printf 'Descargando %s (%s)...\n' "$repo" "$ref"
git clone --depth 1 --branch "$ref" "$repo" "$tmp_dir/source"
(
    cd "$tmp_dir/source"
    go install ./cmd/ai-usage
)

bin_dir=$(go env GOBIN)
if [ -z "$bin_dir" ]; then
    go_path=$(go env GOPATH)
    bin_dir="${go_path%%:*}/bin"
fi
printf '\nai-usage instalado en %s/ai-usage\n' "$bin_dir"
printf 'Comprueba que %s esté en tu PATH.\n' "$bin_dir"
printf 'Para Tern: tern plugin install github.com/fxescandell/fuelcheck/tern-plugin\n'
