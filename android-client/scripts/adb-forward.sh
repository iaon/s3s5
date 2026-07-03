#!/usr/bin/env sh
set -eu
adb forward tcp:${1:-1080} tcp:${2:-1080}
