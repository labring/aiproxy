#!/bin/bash

set -e

if [ ! -f "go.work" ]; then
    echo "go.work file not found in current directory"
    exit 1
fi

echo "Found go.work file, parsing directories..."

directories=()
in_use_block=false

while IFS= read -r line; do
    line=$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

    if [[ -z "$line" || "$line" =~ ^// ]]; then
        continue
    fi

    if [[ "$line" =~ ^use[[:space:]]*\( ]]; then
        in_use_block=true
        continue
    fi

    if [[ "$in_use_block" == true && "$line" =~ ^\) ]]; then
        in_use_block=false
        continue
    fi

    if [[ "$line" =~ ^use[[:space:]]+ ]]; then
        dir=$(echo "$line" | sed 's/^use[[:space:]]*//;s/"//g')
        directories+=("$dir")
        continue
    fi

    if [[ "$in_use_block" == true ]]; then
        dir=$(echo "$line" | sed 's/"//g')
        directories+=("$dir")
    fi
done <go.work

if [ ${#directories[@]} -eq 0 ]; then
    echo "No directories found in go.work file"
    exit 0
fi

echo "Found ${#directories[@]} directories to run golangci-lint --fix:"
for dir in "${directories[@]}"; do
    echo "  - $dir"
done

echo

has_error=false

for dir in "${directories[@]}"; do
    echo "Processing directory: $dir"

    if [ ! -d "$dir" ]; then
        echo "Directory '$dir' does not exist"
        exit 1
    fi

    if [ ! -f "$dir/go.mod" ]; then
        echo "No go.mod found in '$dir', skipping..."
        exit 1
    fi

    # --fix will ignore some issues, so we run it twice
    if (cd "$dir" && golangci-lint run --path-mode abs --fix && golangci-lint run --path-mode abs); then
        echo "Successfully fixed lint issues in '$dir'"
    else
        echo "Failed to fix lint issues in '$dir'"
        has_error=true
    fi

    echo
done

go work sync

if [ "$has_error" = true ]; then
    exit 1
fi
