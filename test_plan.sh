#!/bin/bash
set -e
# Verify things compile
cd apps/api
go build ./...
