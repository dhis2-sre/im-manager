#!/bin/sh
# Keep local and CI tools reproducible; reuse matching installed binaries.
set -eu

tools_bin=$(go env GOBIN)
if [ -z "$tools_bin" ]; then
    tools_bin="$(go env GOPATH)/bin"
fi
install_tool() {
    binary=$1
    module=$2
    package=$3
    version=$4
    if [ -x "$tools_bin/$binary" ] &&
        go version -m "$tools_bin/$binary" | awk -v module="$module" -v version="$version" '
            $1 == "mod" && $2 == module && $3 == version { found = 1 }
            END { exit !found }
        '; then
        echo "$binary $version already installed"
    else
        go install "$package@$version"
    fi
}

install_tool goimports golang.org/x/tools golang.org/x/tools/cmd/goimports v0.50.0
install_tool swagger github.com/go-swagger/go-swagger github.com/go-swagger/go-swagger/cmd/swagger v0.36.6
install_tool git-chglog github.com/git-chglog/git-chglog github.com/git-chglog/git-chglog/cmd/git-chglog v0.15.4
