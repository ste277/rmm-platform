#!/usr/bin/env bash
set -euo pipefail

install -d /Library/RMMAgent
install -m 0755 ./rmm-agent /Library/RMMAgent/rmm-agent
install -m 0644 ./com.company.rmmagent.plist /Library/LaunchDaemons/com.company.rmmagent.plist

launchctl bootstrap system /Library/LaunchDaemons/com.company.rmmagent.plist || true
launchctl enable system/com.company.rmmagent || true
launchctl kickstart -k system/com.company.rmmagent || true
