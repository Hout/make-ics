
#!/usr/bin/env bash
set -euo pipefail

# Convert .po -> go-i18n JSON using bundled Python script.
PO_FILE=${1:-locale/nl_NL/LC_MESSAGES/make_ics.po}
OUTDIR=${2:-locales}
mkdir -p "$OUTDIR"
echo "Converting $PO_FILE -> $OUTDIR"
python3 $(dirname "$0")/convert_po_to_goi18n.py "$PO_FILE" "$OUTDIR"
echo "Done."
