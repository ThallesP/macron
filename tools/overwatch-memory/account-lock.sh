#!/bin/bash
# account-lock.sh take <who and which sandbox> | release <who>
# The test account plays on one machine at a time (PROGRESS.md, "One machine
# at a time"). On the dev machine, the lock is the directory
# ~/.config/macron/steam-test-account.lock with an `owner` file in it. take
# waits, retrying once a minute, while someone else holds it.
set -u
lock="$HOME/.config/macron/steam-test-account.lock"
case "${1:-}" in
  take)
    who=${2:?say who and which sandbox}
    until mkdir "$lock" 2> /dev/null; do
      echo "$(date -u +%T) held by: $(cat "$lock/owner" 2> /dev/null)"
      sleep 60
    done
    echo "$who, from $(date -u +%H:%MZ)" > "$lock/owner"
    echo "taken: $(cat "$lock/owner")"
    ;;
  release)
    who=${2:?say who released it}
    grep -qF "$who" "$lock/owner" 2> /dev/null || { echo "not held by $who: $(cat "$lock/owner" 2> /dev/null)" >&2; exit 1; }
    rm -r "$lock" && echo released
    ;;
  *) echo "usage: account-lock.sh take <who> | release <who>" >&2; exit 2 ;;
esac
