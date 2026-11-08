# LianT

[简体中文](README.zh-CN.md) | English

A self-hosted client and service for operating chat-platform bots (QQ first; Telegram next).

- Design and decisions: [DESIGN.md](docs/DESIGN.md)
- Wire protocol (draft): [PROTOCOL.md](docs/PROTOCOL.md)
- UI rules (web console): [UI-DESIGN.md](docs/UI-DESIGN.md)
- Deployment: [DEPLOY.md](docs/DEPLOY.md)
- Security model: [SECURITY.md](docs/SECURITY.md)
- Release rules: [RELEASING.md](docs/RELEASING.md)

## License

This project is licensed as a whole under the **GNU Affero General Public License v3.0 (AGPL-3.0)**,
covering `service/`, `web/`, and every other file in the repository.

The management console is no longer a separately distributed, permissively licensed client:
it is built into the Service binary and served as a web page. Offering a modified version
over a network therefore requires publishing the corresponding source, as AGPL-3.0 states.

The full terms are in the root [LICENSE](LICENSE).
