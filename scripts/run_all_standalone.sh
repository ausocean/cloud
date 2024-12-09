#!/bin/bash

# DESCRIPTION
#   Automates the process of running all AusOcean cloud services in standalone for local testing.
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

# Set the paths to your services
OCEANBENCH_PATH="../cmd/oceanbench"
OCEANTV_PATH="../cmd/oceantv"
OCEANCRON_PATH="../cmd/oceancron"
DATABLUE_PATH="../cmd/datablue"

# Set the file store path relative to oceanbench.
FILESTORE_PATH="../oceanbench/store"

# Set the URLs for the services.
TV_URL="http://localhost:8082"
CRON_URL="http://localhost:8081"

# Check if the YOUTUBE_API_CREDENTIALS environment variable is set.
if [[ -z "${YOUTUBE_API_CREDENTIALS}" ]]; then
    echo "Warning: YOUTUBE_API_CREDENTIALS environment variable is not set, you won't be able to stream to YouTube."
    echo "Ensure you have downloaded the credentials and set the variable."
    echo "For example: export YOUTUBE_API_CREDENTIALS=~/.config/youtube_api_credentials.json"
else
    echo "YOUTUBE_API_CREDENTIALS is set to: ${YOUTUBE_API_CREDENTIALS}"
fi

# Function to run a service and log its output.
run_service() {
    local service_name="$1"
    local service_path="$2"
    local command="$3"

    echo "Starting $service_name in directory $service_path..."
    (
        cd "$service_path"
        
        # Attempt to run npm run build if it's OceanBench.
        if [[ "$service_name" == "OceanBench" ]]; then
            npm run build || echo "Warning: npm run build failed for OceanBench"
        fi

        # Run the service.
        $command 2>&1 | sed "s/^/[$service_name] /"
    ) &
    local pid=$!
    echo "$service_name started with PID $pid"
}

# Trap to clean up background processes on exit.
cleanup() {
    echo "Stopping all services..."
    pkill -P $$
    echo "All services stopped."
    exit
}

trap cleanup SIGINT SIGTERM

# Run all services.
run_service "OceanBench" "$OCEANBENCH_PATH" "go run . --standalone --tvurl $TV_URL --cronurl $CRON_URL"
run_service "OceanTV" "$OCEANTV_PATH" "go run . --standalone --filestore $FILESTORE_PATH"
run_service "OceanCron" "$OCEANCRON_PATH" "go run . --standalone --filestore $FILESTORE_PATH"
run_service "DataBlue" "$DATABLUE_PATH" "go run . --standalone --filestore $FILESTORE_PATH"

# Wait for all background processes.
echo "All services are running. Press Ctrl+C to stop them."
wait
