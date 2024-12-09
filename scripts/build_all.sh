#!/bin/bash

# DESCRIPTION
#   Automates the process of building all AusOcean cloud services for local testing.
# AUTHORS
#   Trek Hopton <trek@ausocean.org>

# LICENSE
#   Copyright (C) 2024 the Australian Ocean Lab (AusOcean).

#   This is free software: you can redistribute it and/or modify it
#   under the terms of the GNU General Public License as published by
#   the Free Software Foundation, either version 3 of the License, or
#   (at your option) any later version.

#   This is distributed in the hope that it will be useful, but WITHOUT
#   ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
#   or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public
#   License for more details.

#   You should have received a copy of the GNU General Public License
#   in gpl.txt. If not, see http://www.gnu.org/licenses/.

# Set the Go workspace (adjust paths as needed).
PROJECT_DIRS=(
    "../cmd/oceanbench"
    "../cmd/oceantv"
    "../cmd/oceancron"
    "../cmd/datablue"
)

# Check if Go is installed.
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go and try again."
    exit 1
fi

# Loop over each directory and build the binaries.
for dir in "${PROJECT_DIRS[@]}"; do
    echo "Building binary for $dir..."
    
    # Ensure the directory exists.
    if [ -d "$dir" ]; then
        # Navigate to the directory.
        cd "$dir" || exit
        
        # Run `go build` to build the binary.
        if go build -o "$(basename "$dir")" .; then
            echo "Successfully built binary for $dir"
        else
            echo "Failed to build binary for $dir"
            exit 1
        fi

        # Return to the previous directory.
        cd - || exit
    else
        echo "Directory $dir does not exist. Skipping..."
    fi
done

echo "All binaries have been built."