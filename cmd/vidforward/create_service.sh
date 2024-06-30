#!/bin/bash
# This script is utilised by Makefile for creation of a systemd service. The
# service lines are stored in a string allowing us to substitute the GOPATH into
# the ExecStart path of the service file.

if [ $# -ne 1 ]; then
  echo "incorrect number of arguments, expected run script and binary directories"
  exit 1
fi

# This corresponds to the binary dir. e.g. /src/github.com/ausocean/av/cmd/vidforward.
bin_dir=$1

# This is the IP we'll run the host on.
host=$(hostname -I | awk '{print $1}')

# Get the bin name (assuming this is at the end of the bin_dir).
bin_name=$(basename $bin_dir)

# First find the user that corresponds to this path (which is assumed to be at the
# base of the current working directory).
in=$(pwd)
arr_in=(${in//// })
gopath_user=${arr_in[1]}

# We can now form the gopath from the obtained user.
gopath="/home/$gopath_user/go"

# Here are the lines that will go into the rv.service file. We'll set the
# ExecStart field as the GOPATH we've obtained + the passed run script dir.
service="
[Unit]
Description=vidforward service for forwarding video to youtube

[Service]
Type=notify
ExecStart=$gopath$bin_dir/vidforward -host $host
WatchdogSec=30s
Restart=on-failure

[Install]
WantedBy=multi-user.target
"

# The service name will just use the bin name.
service_name="$bin_name.service"

# Now overwrite the service if it exists, or create the service then write.
service_dir=/etc/systemd/system/$service_name
if [ -f $service_dir ]; then
  echo "$service" > $service_dir
else
  touch $service_dir
  echo "$service" > $service_dir
fi
