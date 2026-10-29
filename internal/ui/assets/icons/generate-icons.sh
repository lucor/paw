#!/usr/bin/env bash
set -euo pipefail

readonly DEFAULT_LUCIDE_VERSION="1.54.0"
readonly REQUIRED_RESVG_VERSION="0.48.1"
readonly ICON_SIZE=64
readonly ICONS_ACTIVE="circle-plus vault timer settings"
readonly ICONS_LINK=""
readonly ICONS="folder-open cloud arrow-left chevron-right chevron-down chevron-up download dices lock lock-open ellipsis-vertical plus search file-text globe key-round rotate-ccw-key file-key terminal shield-check file-input file-output lock-keyhole copy eye eye-off square-pen trash external-link"
readonly ICONS_DANGER="trash"
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd -- "$SCRIPT_DIR/../../../../.." && pwd)"

lucide_version="$DEFAULT_LUCIDE_VERSION"
resvg_bin="${RESVG_BIN:-}"
oxipng_bin="${OXIPNG_BIN:-}"
output_dir="$SCRIPT_DIR"

usage() {
	cat <<'EOF'
Usage: generate-icons.sh [--version TAG] [--resvg PATH] [--oxipng PATH] [--output-dir PATH]

Downloads official Lucide SVGs and renders Paw v2's 64x64 PNG icon set.
Lucide defaults to 1.54.0 and resvg must be version 0.48.1.
EOF
}

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

while (($#)); do
	case "$1" in
	--version)
		(($# >= 2)) || fail "--version requires a tag"
		lucide_version="${2#v}"
		shift 2
		;;
	--resvg)
		(($# >= 2)) || fail "--resvg requires a path"
		resvg_bin="$2"
		shift 2
		;;
	--oxipng)
		(($# >= 2)) || fail "--oxipng requires a path"
		oxipng_bin="$2"
		shift 2
		;;
	--output-dir)
		(($# >= 2)) || fail "--output-dir requires a path"
		output_dir="$2"
		shift 2
		;;
	-h|--help)
		usage
		exit 0
		;;
	*)
		fail "unknown argument: $1"
		;;
	esac
done

if [[ -z "$resvg_bin" ]]; then
	resvg_bin="$(command -v resvg || true)"
	if [[ -z "$resvg_bin" && -x "$REPO_ROOT/resvg" ]]; then
		resvg_bin="$REPO_ROOT/resvg"
	fi
fi
[[ -n "$resvg_bin" && -x "$resvg_bin" ]] || fail "resvg CLI not found; install resvg 0.48.1 or set RESVG_BIN"
actual_resvg_version="$("$resvg_bin" --version)"
[[ "$actual_resvg_version" == "$REQUIRED_RESVG_VERSION" ]] || \
	fail "expected resvg $REQUIRED_RESVG_VERSION, found $actual_resvg_version"
if [[ -z "$oxipng_bin" ]]; then
	oxipng_bin="$(command -v oxipng || true)"
fi
[[ -n "$oxipng_bin" && -x "$oxipng_bin" ]] || fail "oxipng CLI not found; install oxipng or set OXIPNG_BIN"
actual_oxipng_version="$("$oxipng_bin" --version | awk '{print $2}')"

if [[ "$output_dir" != /* ]]; then
	output_dir="$REPO_ROOT/$output_dir"
fi
mkdir -p "$output_dir/light" "$output_dir/dark" "$output_dir/active" "$output_dir/danger"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

download_icon() {
	local icon="$1"
	local destination="$2"
	local url="https://raw.githubusercontent.com/lucide-icons/lucide/$lucide_version/icons/$icon.svg"
	local status
	status="$(curl -sSL --retry 3 --connect-timeout 15 -o "$destination" -w '%{http_code}' "$url")" || \
		fail "could not download Lucide icon $icon from $url"
	if [[ "$status" != 200 ]]; then
		rm -f "$destination"
		return 1
	fi
	return 0
}

validate_svg() {
	local icon="$1"
	local source="$2"
	local expected
	for expected in \
		'width="24"' \
		'height="24"' \
		'viewBox="0 0 24 24"' \
		'fill="none"' \
		'stroke="currentColor"' \
		'stroke-width="2"' \
		'stroke-linecap="round"' \
		'stroke-linejoin="round"'; do
		grep -Fq "$expected" "$source" || fail "unexpected SVG source properties for $icon (missing $expected)"
	done
	local color_count
	color_count="$(grep -o 'currentColor' "$source" | wc -l | tr -d ' ')"
	[[ "$color_count" -gt 0 ]] || fail "Lucide SVG has no currentColor value to replace for $icon"
}

render_icon() {
	local filename="$1"
	local source="$2"
	local variant="$3"
	local color="$4"
	local colored="$tmp_dir/$filename-$variant.svg"
	local output="$output_dir/$variant/$filename.png"
	local round_trip="$tmp_dir/$filename-$variant-round-trip.svg"
	local header
	local w1 w2 w3 w4 h1 h2 h3 h4 bit_depth color_type
	local width height

	sed "s/currentColor/$color/g" "$source" > "$colored"
	sed "s/$color/currentColor/g" "$colored" > "$round_trip"
	cmp -s "$source" "$round_trip" || fail "stroke replacement changed SVG geometry for $filename"

	"$resvg_bin" --width "$ICON_SIZE" --height "$ICON_SIZE" \
		--shape-rendering geometricPrecision "$colored" "$output"
	[[ -s "$output" ]] || fail "resvg generated an empty PNG for $filename ($variant)"
	"$oxipng_bin" --opt 2 --strip safe --nb --nc --quiet "$output"

	# Read the PNG IHDR directly to verify 64x64, 8-bit RGBA output.
	header="$(od -An -j16 -N10 -tu1 "$output")"
	read -r w1 w2 w3 w4 h1 h2 h3 h4 bit_depth color_type <<<"$header"
	width=$((w1 << 24 | w2 << 16 | w3 << 8 | w4))
	height=$((h1 << 24 | h2 << 16 | h3 << 8 | h4))
	[[ "$width" == "$ICON_SIZE" && "$height" == "$ICON_SIZE" ]] || \
		fail "unexpected PNG dimensions for $filename ($variant): ${width}x${height}"
	[[ "$bit_depth" == 8 && "$color_type" == 6 ]] || \
		fail "PNG for $filename ($variant) is not 8-bit RGBA"
	printf 'Generated %s/%s.png (%s)\n' "$variant" "$filename" "$color"
}

color_for_variant() {
	case "$1" in
	light) printf '#565656' ;;
	dark) printf '#F3F3F3' ;;
	active) printf '#0B8FF3' ;;
	danger) printf '#E5484D' ;;
	*) fail "unknown color variant: $1" ;;
	esac
}

generate_group() {
	local icon="$1"
	local variants="$2"
	local source="$tmp_dir/$icon.svg"
	local variant
	local color

	download_icon "$icon" "$source" || fail "Lucide icon $icon was not found in release $lucide_version (HTTP error)"
	validate_svg "$icon" "$source"

	for variant in $variants; do
		color="$(color_for_variant "$variant")"
		render_icon "$icon" "$source" "$variant" "$color"
	done
}

for variant in light dark active danger; do
	find "$output_dir/$variant" -maxdepth 1 -type f -name '*.png' -delete
done

for icon in $ICONS_ACTIVE; do
	generate_group "$icon" "light dark active"
done
for icon in $ICONS_LINK; do
	generate_group "$icon" "light dark active"
done
for icon in $ICONS; do
	generate_group "$icon" "light dark"
done
for icon in $ICONS_DANGER; do
	generate_group "$icon" "danger"
done

license_url="https://raw.githubusercontent.com/lucide-icons/lucide/$lucide_version/LICENSE"
license_status="$(curl -sSL --retry 3 --connect-timeout 15 -o "$output_dir/LICENSE" -w '%{http_code}' "$license_url")" || \
	fail "could not download the Lucide license from $license_url"
[[ "$license_status" == 200 ]] || fail "Lucide license was not found at release $lucide_version"

cat > "$output_dir/README.md" <<EOF
# Paw Lucide icon assets

These PNGs are generated from official [Lucide Icons](https://github.com/lucide-icons/lucide) SVGs at release \`$lucide_version\`. The SVG path geometry is preserved; only the stroke color is changed before rendering.

- PNG: 64 x 64 pixels, 8-bit RGBA, transparent background.
- Renderer: [resvg](https://github.com/linebender/resvg) $REQUIRED_RESVG_VERSION with geometric precision.
- Lossless PNG optimization: oxipng $actual_oxipng_version.
- Light stroke: #565656.
- Dark stroke: #F3F3F3.
- Active stroke: #0B8FF3.
- Danger stroke: #E5484D.
Regenerate from the repository root with the mise run generate-icons task. To choose another Lucide release, pass --version TAG to internal/ui/assets/icons/generate-icons.sh.

The Lucide license and copyright notices are preserved in [LICENSE](LICENSE).
EOF

printf 'Generated %s PNG icons using Lucide %s, resvg %s, and oxipng %s\n' \
	"$(find "$output_dir/light" "$output_dir/dark" "$output_dir/active" "$output_dir/danger" -maxdepth 1 -type f -name '*.png' | wc -l | tr -d ' ')" \
	"$lucide_version" "$actual_resvg_version" "$actual_oxipng_version"
