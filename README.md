# LianT

[简体中文](README.zh-CN.md) | English

LianT is a self-hosted **Bot Engine** for chat-platform bots (QQ first; Telegram next),
with a Flutter client for operating it.

- **Engine** (`service/`): Go, WSS protocol v1, SQLite, 7-day message buffer. It serves
  `/v1/ws` and signed media links only — no embedded web console.
- **Client** (`manager/`): Flutter. **Android is the only shipped target**; the web build is
  a local preview surface only and is not published. No desktop builds.
  Conversations, bots, devices and audit logs are part of the same protocol surface.

## Quick start

Install the Service on a Linux host (latest pre-release binary, systemd unit, config init):

```bash
curl -fsSL https://raw.githubusercontent.com/Bailensn/LianT/main/scripts/install.sh | sudo bash
```

Use `LIANT_DIR=/opt/liant LIANT_PORT=8443` to override the defaults, or pass a local binary:
`sudo bash scripts/install.sh ./LianT-linux-amd64`.

Then open the Android client, paste the pairing info printed by `LianT pair`, and confirm
on the server terminal. The Service serves `/v1/ws` and `/v1/media/` only; there is no web console.

## Documents

- Index: [docs/README.md](docs/README.md)
- Design and decisions: [DESIGN.md](docs/DESIGN.md)
- Wire protocol: [PROTOCOL.md](docs/PROTOCOL.md)
- UI rules (Flutter): [UI-DESIGN.md](docs/UI-DESIGN.md)
- Deployment: [DEPLOY.md](docs/DEPLOY.md)
- Security model: [SECURITY.md](docs/SECURITY.md)
- Release rules: [RELEASING.md](docs/RELEASING.md)
- Handoff / current status: [HANDOFF.md](docs/HANDOFF.md)

## Repository layout

```text
LianT/
├── service/   Go service (API only)
├── manager/   Flutter client
├── docs/      design, protocol and operational docs
├── deploy/    systemd unit
└── scripts/   install and protocol-drift checks
```

## License

This project is licensed as a whole under the **GNU Affero General Public License v3.0 (AGPL-3.0)**,
covering `service/`, `manager/`, and every other file in the repository.

Offering a modified version over a network therefore requires publishing the corresponding
source, as AGPL-3.0 states. The full terms are in the root [LICENSE](LICENSE).
