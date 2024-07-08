#!/bin/bash

# Enter the static file store.
cd dl

# Check all device types.
types=$(ls)
for type in $types; do
    echo "Checking $type versions"

    # Enter device directory.
    cd $type

    # Check all component versions.
    components=$(ls)
    for comp in $components; do
        cd $comp

        # Get current symlink value.
        symlink=$(readlink @latest)

        # Get the actual latest.
        latest=$(ls | tail -n 1)

        # Compare and update symlink if it has lagged.
        if [[ "$symlink" != "$latest" ]]; then
            echo -e "\ttry to update symlink for $comp;\n\t\tsym:$symlink latest:$latest"
            ln -sfn $latest @latest
        fi

        cd ..
    done

    cd ..
done