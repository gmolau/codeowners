#!/bin/sh
#
# Entrypoint for the Dockerfile for use as GitHub Action

# /github/workspace is to where GitHub maps the repo and sets the workdir
/codeowners /github/workspace > /github/workspace/.github/CODEOWNERS
