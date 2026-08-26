#

love:	example.txt
	export DEBUG=false; jq -c < example.jscon | ~/bin/node2vim.sh nodejs jscon.js

example.txt:	example.jscon
	jq 'to_entries[] | ((.key | tostring) + " " + (.value | tostring))' < '$<' > '$@'

