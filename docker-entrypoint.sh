#!/bin/sh
set -eu

set -- /app/tgautodown -cfg "${TG_CFG:-/app/data/config.json}"

if [ "${TG_CHANNEL:-}" != "" ]; then
  set -- "$@" -names "$TG_CHANNEL"
fi

if [ "${TG_PROXY:-}" != "" ]; then
  set -- "$@" -proxy "$TG_PROXY"
fi

if [ "${TG_F2A:-}" != "" ]; then
  set -- "$@" -f2a "$TG_F2A"
fi

if [ "${TG_RETRYCNT:-}" != "" ]; then
  set -- "$@" -retrycnt "$TG_RETRYCNT"
fi

exec "$@"
