#!/usr/bin/env sh
set -eu

umask 077

cmd="${1:-}"

usage() {
  cat <<'EOF'
Usage:
  ./generate-dev-certs.sh gen-ca [ca_cn]
  ./generate-dev-certs.sh gen-node <node_cn> <san> [ca_dir]

Examples:
  ./generate-dev-certs.sh gen-ca tokmesh-cluster-dev-ca
  ./generate-dev-certs.sh gen-node tokmesh-node-1 "DNS:tokmesh-node-1,IP:10.0.0.10" .
EOF
}

case "${cmd}" in
  gen-ca)
    ca_cn="${2:-tokmesh-cluster-dev-ca}"
    openssl req -x509 -newkey rsa:2048 -days 3650 -nodes \
      -keyout ca.key -out ca.crt -subj "/CN=${ca_cn}"
    echo "Generated CA: ca.crt ca.key (dev-only)"
    ;;
  gen-node)
    node_cn="${2:-}"
    san="${3:-}"
    ca_dir="${4:-.}"
    if [ -z "${node_cn}" ] || [ -z "${san}" ]; then
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
      -keyout node.key -out node.csr -subj "/CN=${node_cn}"

    openssl x509 -req -days 3650 -in node.csr -CA "${ca_dir}/ca.crt" -CAkey "${ca_dir}/ca.key" -CAcreateserial \
      -out node.crt -extfile "${extfile}"

    rm -f node.csr "${ca_dir}/ca.srl" 2>/dev/null || true
    echo "Generated node cert: node.crt node.key (dev-only)"
    ;;
  *)
    usage
    exit 2
    ;;
esac
