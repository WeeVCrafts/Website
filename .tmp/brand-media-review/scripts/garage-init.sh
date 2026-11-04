#!/bin/sh
set -eu

config_file="${GARAGE_CONFIG_FILE:-/etc/garage.toml}"
rpc_addr="${GARAGE_RPC_ADDR:-garage:3901}"
node_id=""
until [ -n "$node_id" ]; do
  node_id="$(/usr/local/bin/garage -c "$config_file" node id 2>/dev/null | sed 's/@.*//' | tr -d '\r\n' || true)"
  [ -n "$node_id" ] || sleep 1
done
rpc_host="${node_id}@${rpc_addr}"
node_short="$(printf '%s' "$node_id" | cut -c1-16)"

until /usr/local/bin/garage -c "$config_file" -h "$rpc_host" status >/dev/null 2>&1; do
  sleep 1
done

layout="$('/usr/local/bin/garage' -c "$config_file" -h "$rpc_host" layout show)"
if ! printf '%s\n' "$layout" | grep -q "$node_short"; then
  /usr/local/bin/garage -c "$config_file" -h "$rpc_host" layout assign -z dc1 -c 1G "$node_id"
  /usr/local/bin/garage -c "$config_file" -h "$rpc_host" layout apply --version 1
fi

if ! /usr/local/bin/garage -c "$config_file" -h "$rpc_host" key info wecratfs >/dev/null 2>&1; then
  /usr/local/bin/garage -c "$config_file" -h "$rpc_host" key import --yes -n wecratfs \
    GK0123456789abcdef01234567 \
    0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
fi

if ! /usr/local/bin/garage -c "$config_file" -h "$rpc_host" bucket info wecratfs-media >/dev/null 2>&1; then
  /usr/local/bin/garage -c "$config_file" -h "$rpc_host" bucket create wecratfs-media
fi

/usr/local/bin/garage -c "$config_file" -h "$rpc_host" bucket allow wecratfs-media --read --write --key wecratfs
