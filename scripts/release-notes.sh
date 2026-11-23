#!/bin/sh
# Prints the body of the section for a version from a Keep a Changelog file,
# for use as GitHub release notes.
#
# Usage:
#   release-notes.sh VERSION [CHANGELOG]   e.g. release-notes.sh 0.1.0
#   release-notes.sh --check [CHANGELOG]   validate the file structure only
#
# Exits with an error if the section is missing or empty, so a release is never
# published without notes.
set -eu

if [ "${1:-}" = "--check" ]; then
	file="${2:-CHANGELOG.md}"
	grep -q '^## \[Unreleased\]' "$file" || {
		echo "$file: missing the ## [Unreleased] section" >&2
		exit 1
	}
	exit 0
fi

version="${1:?usage: release-notes.sh VERSION [CHANGELOG]}"
file="${2:-CHANGELOG.md}"

notes="$(awk -v header="## [$version]" '
	index($0, header) == 1 { found = 1; next }
	found && /^## \[/ { exit }
	found && /^\[[^]]+\]: / { exit }
	found { print }
' "$file" | sed -e '/./,$!d')"

if [ -z "$(printf '%s' "$notes" | tr -d '[:space:]')" ]; then
	echo "$file: no notes for version $version" >&2
	exit 1
fi

printf '%s\n' "$notes"
