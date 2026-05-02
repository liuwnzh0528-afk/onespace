#!/bin/sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

init_repo() {
  repo="$1"
  if [ ! -d "$repo/.git" ]; then
    git -C "$repo" init -b main >/dev/null 2>&1 || git -C "$repo" init >/dev/null
  fi
  git -C "$repo" config user.email "onespace-smoke@example.invalid"
  git -C "$repo" config user.name "Onespace Smoke"
  git -C "$repo" add .
  if ! git -C "$repo" diff --cached --quiet; then
    if git -C "$repo" rev-parse --verify HEAD >/dev/null 2>&1; then
      git -C "$repo" commit -m "smoke update" >/dev/null
    else
      git -C "$repo" commit -m "smoke initial" >/dev/null
    fi
  fi
}

init_repo "$ROOT/smoke-go/repos/user-api"
init_repo "$ROOT/smoke-java/repos/order-api"
