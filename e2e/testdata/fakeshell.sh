#!/bin/sh
# Fake $SHELL used by e2e tests for the `new` and `shell` commands.
#
# internal/shell.Spawn replaces the workstreams process with $SHELL via
# syscall.Exec — it does not fork, so a real interactive shell would hang
# waiting for input. This script stands in for it: it echoes the injected
# workstream env vars and the working directory, then exits immediately, so
# tests can assert env injection without ever spawning a real shell.
echo "WORKSTREAM_BRANCH=$WORKSTREAM_BRANCH"
echo "WORKSTREAM_PATH=$WORKSTREAM_PATH"
echo "PWD=$(pwd)"
exit 0
