#!/bin/bash
USAGE="
Usage: $0 [-d] [-b] <projectID>

Options:
  -d  : Development Mode
        Deploys with the --no-promote flag and sets the deployment version to 'dev'.
        This will get deployed to dev-dot-<projectID>.ts.r.appspot.com.

  -b  : Build Mode
        Runs 'npm run build' to build the project before deploying.

  <projectID>  : Required Argument
        The Google Cloud project ID for deployment, passed to the 'gcloud app deploy' command.

Description:
This script automates the deployment of a project to Google Cloud. It supports two modes:
1. Development Mode ('-d'): Deploys with the 'dev' version and no promotion.
2. Build Mode ('-b'): Builds the project using 'npm run build' before deployment.
By default, the script extracts the version from 'main.go' (using the middle number in the version).
The script will deploy the project to Google Cloud using the specified projectID and version.

Example Usage:
  1. Deploy with build and version extraction:
     ./deploy.sh -b myProjectID

  2. Deploy in development mode:
     ./deploy.sh -d myProjectID

  3. Only build the project (no deployment):
     ./deploy.sh -b

Notes:
- Ensure you have the correct Google Cloud credentials and project setup.
- This script must be called from the same directory as the project files. (ie cmd/oceanbench)
"

# Displays the usage for the script.
display_help() {
    echo "$USAGE"
}

find_yaml() {
    local FILENAME="$1.yaml"
    local CURRENT_DIR=$(pwd)

    # Traverse back up the tree looking for YAML.
    while [[ "$CURRENT_DIR" != "/" ]]; do
        # Check if the file exists in the current directory
        if [[ -f "$CURRENT_DIR/$FILENAME" ]]; then
            echo "$CURRENT_DIR/$FILENAME"
            exit 0
        fi
        # Move to the parent directory
        CURRENT_DIR=$(dirname "$CURRENT_DIR")
    done
    echo "Couldn't find $FILENAME file"
    exit 1
}

# Check if --help is passed
if [[ "$1" == "--help" ]]; then
    display_help
    exit 0
fi

DEVELOPMENT=false
BUILD=false

# Check options.
while getopts ":db" opt; do
    case ${opt} in
        d)
            # Deploy to a development environment.
            DEVELOPMENT=true
            ;;
        b)
            # Use npm run build.
            BUILD=true
            ;;
        ?)
            echo "Invalid option: -${OPTARG}."
            display_help
            exit 1
            ;;
  esac
done

# Remove options from arguments.
shift $(($OPTIND - 1))

if $BUILD; then
    if ! npm run build; then
      read -r -p "Build failed, do you want to continue? [y/N] " response
      case "$response" in
        [yY][eE][sS]|[yY])
            echo "Continuing despite build failure..."
            ;;
        *)
          echo "Exiting due to build failure."
          exit 1
          ;;
      esac
    fi
fi

# Find the YAML file in the tree.
YAML=$(find_yaml $1)
if [[ $? -ne 0 ]]; then
    echo "Error: YAML file not found."
    exit 1
fi

if $DEVELOPMENT; then
    PROMOTE="--no-promote"
    DEPLOYMENT_VERSION="dev"

    TEMP="$(dirname "$YAML")/temp_$(basename "$YAML")"
    cp $YAML $TEMP
    YAML=$TEMP

    sed -i "/^  OAUTH2_CALLBACK/d" $YAML
    sed -i "s/# OAUTH2_CALLBACK/\OAUTH2_CALLBACK/g; s/# DEVELOPMENT/\DEVELOPMENT/g" $YAML
else
    # Get the version from main.go in the local directory.
    echo "Looking for version number in main.go"
    version_line=$(grep -oP 'version\s+=\s+"v\d+\.\d+\.\d+"' "main.go")
    echo "Found version in main.go: $(echo "$version_line" | cut -d '"' -f 2)"

    PROMOTE="--promote"
    DEPLOYMENT_VERSION=$(echo "$version_line" | cut -d '.' -f 2)
fi

echo "Deploying to version: $DEPLOYMENT_VERSION"

# Deploy using app.yaml file in cloud/ directory
gcloud "app" "deploy" "--project=$1" "--version=$DEPLOYMENT_VERSION" "$PROMOTE" "--no-cache" "$YAML"

if $DEVELOPMENT; then
  rm $YAML
fi
