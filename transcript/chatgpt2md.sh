#!/bin/bash
#
# vim: ft=bash
#
# convert *.chatgpt to *.md
#
# *.chatgpt is a meta file consisting of paragraphs of:
#
# - Your input (copy and paste)
# - ChatGPT URL (copied)
#
# URLs must be downloaded by you first separately and stored as a file named after the path part.
# (so just do: wget URL)

# Note that this probably will need update, soon
extract()
{
  # extract the interesting part of the metadata
  # (this is not used, but perhaps interesting)
  [ "$1.p.json" -nt "$1" ] ||
	awk 'BEGIN { RS="<script" } / type="application\/json" / { sub(/^[^>]*>{/,"{"); gsub(/}<\/script>.*$/,"}"); print $0 }' "$1" |
	jq |
	tee "$1.r.json" |
	jq -r .statsigPayload |
	jq |
	tee "$1.q.json" |
	jq '[.[] | objects[] | objects | select(.rule_id == "default").value | objects | [to_entries[] | select(.value | type == "string")] | select(length>0) | from_entries]' > "$1.p.json";
  # extract the interesting part with the answer
  # (sadly the full prompt is not available)
  [ "$1.d.json" -nt "$1" ] ||
	awk 'BEGIN { RS="<script" } /enqueue\("\[\{/ { sub(/^[^{]*\("/,"\""); gsub(/");<\/script>.*$/,"\""); print $0; exit(0) }' "$1" |
	jq -r |
	jq > "$1.d.json";

  # decompress the JSON and extract the answer
  # (Fortunately this already is is markdown formatted)
  [ "$1.md" -nt "$1.d.json" ] ||
	jq -c < "$1.d.json" |
	js js/jscon.js |
	jq -r '.. | .parts? // empty | .[]' > "$1.md";

  # output markdown
  cat "$1.md"
}

isopen=false
nr=0
open()
{
  $isopen && return;
  let nr++
  printf -- '---\n# Prompt %d\n\n' "$nr"
  isopen=:
}
close()
{
  $isopen || return 0
  printf -- '---\n\n'
  isopen=false
}

convert()
{
  while read -ru6 line
  do
	case "$line" in
	(https://chatgpt.com/*)
		close
		# extract the (manually) downloaded URLs (use: wget)
		extract "${line##*/}"
		continue
		;;
	('')	echo
		continue
		;;
	esac
	open
	# Output our prompts from the file
	echo "> $line"
  done 6<"$1" >"${1%.chatgpt}.md"
}

#[ 0 = $# ] && set -- *.chatgpt
for a
do
	convert "$a"
done

