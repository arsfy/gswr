#!/bin/bash
set -e

rm -rf ./internal/webui/dist
cd web
pnpm run build
mv dist ../internal/webui
cd ..