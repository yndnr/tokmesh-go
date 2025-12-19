#!/usr/bin/env sh
set -eu

umask 077

cmd="${1:-}"

usage() {
  cat <<'EOF'
Usage:
  ./generate-dev-certs.sh gen-ca [ca_cn]
  ./generate-dev-certs.sh gen-server <server_cn> <san> [ca_dir]

Examples:
  ./generate-dev-certs.sh gen-ca tokmesh-dev-ca
  ./generate-dev-certs.sh gen-server tokmesh-server "DNS:localhost,IP:127.0.0.1" .
EOF
}

case "${cmd}" in
  gen-ca)
    ca_cn="${2:-tokmesh-dev-ca}"
    openssl req -x509 -newkey rsa:2048 -days 3650 -nodes \
      -keyout ca.key -out ca.crt -subj "/CN=${ca_cn}"
    echo "Generated CA: ca.crt ca.key (dev-only)"
    ;;
  gen-server)
    server_cn="${2:-}"
    san="${3:-}"
    ca_dir="${4:-.}"
    if [ -z "${server_cn}" ] || [ -z "${san}" ]; then
      usage
      exit 2
    fi
    if [ ! -f "${ca_dir}/ca.crt" ] || [ ! -f "${ca_dir}/ca.key" ]; then
      echo "Error: missing CA files in ${ca_dir} (need ca.crt and ca.key)" >&2
      exit 2
    fi

    extfile="$(mktemp)"
    trap 'rm -f "$extfile"' EXIT
    printf "subjectAltName=%s\n" "${san}" > "${extfile}"

    openssl req -newkey rsa:2048 -nodes \
      -keyout server.key -out server.csr -subj "/CN=${server_cn}"

    openssl x509 -req -days 3650 -in server.csr -CA "${ca_dir}/ca.crt" -CAkey "${ca_dir}/ca.key" -CAcreateserial \
      -out server.crt -extfile "${extfile}"

    rm -f server.csr "${ca_dir}/ca.srl" 2>/dev/null || true
    echo "Generated server cert: server.crt server.key (dev-only)"
    ;;
  *)
    usage
    exit 2
    ;;
esac
