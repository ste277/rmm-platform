# Architecture Overview

The platform is split into:

- A cross-platform endpoint agent
- A backend control plane
- A web dashboard
- Shared protocol contracts in `proto/`

The agent maintains an outbound secure connection to the backend and sends heartbeats, inventory, telemetry, and compliance data.
