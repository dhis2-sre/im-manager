#!/bin/sh

watch --differences --interval 1 \
    curl --silent "$IM_HOST/sigterm"
