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

echo "Found ${#directories[@]} directories to update:"
for dir in "${directories[@]}"; do
    echo "  - $dir"
done

echo

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

    if (cd "$dir" && go get -u && go mod tidy); then
        echo "Successfully updated dependencies in '$dir'"
    else
        echo "Failed to update dependencies in '$dir'"
        exit 1
    fi

    echo
done

go work sync
