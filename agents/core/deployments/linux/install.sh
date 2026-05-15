#!/usr/bin/env bash
set -euo pipefail

install -d /opt/rmm-agent
install -d /etc/rmm-agent
install -m 0755 ./rmm-agent /opt/rmm-agent/rmm-agent
install -m 0644 ./rmm-agent.service /etc/systemd/system/rmm-agent.service

cat >/etc/rmm-agent/rmm-agent.env <<EOF
RMM_SERVER_URL=${RMM_SERVER_URL:-https://rmm.example.com}
RMM_TENANT_ID=${RMM_TENANT_ID:-}
RMM_AGENT_ID=${RMM_AGENT_ID:-}
EOF

systemctl daemon-reload
systemctl enable --now rmm-agent
