#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
ubiqdir="$workspace/src/github.com/ubiq"
if [ ! -L "$ubiqdir/spectrum-backend" ]; then
    mkdir -p "$ubiqdir"
    cd "$ubiqdir"
    ln -s ../../../../../. spectrum-backend
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$ubiqdir/spectrum-backend"
PWD="$ubiqdir/spectrum-backend"

# Launch the arguments with the configured environment.
exec "$@"
