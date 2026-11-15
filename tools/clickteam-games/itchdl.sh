#!/bin/bash
# itchdl.sh <page-url> <upload-id> <out-file>: the page's own Download button
# for a free (minimum price 0) itch.io game: a POST to /file/<id> returns a
# signed storage URL valid for 60 s.
set -eu
page=$1 id=$2 out=$3
UA='Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0'
jar=$(mktemp)
trap 'rm -f "$jar"' EXIT
h=$(curl -s -c "$jar" -b "$jar" -A "$UA" "$page")
tok=$(grep -o 'name="csrf_token" value="[^"]*"' <<<"$h" | head -1 | sed 's/.*value="//;s/"$//')
r=$(curl -s -c "$jar" -b "$jar" -A "$UA" -X POST --data-urlencode "csrf_token=$tok" "$page/file/$id?source=view_game&as_props=1&after_download_lightbox=1")
url=$(python3 -I -c 'import json,sys;print(json.load(sys.stdin)["url"])' <<<"$r")
curl -sSL -A "$UA" "$url" -o "$out"
echo "$id $(shasum -a 256 <"$out" | cut -c1-64) $(wc -c <"$out" | tr -d " ")"
