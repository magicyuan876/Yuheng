# Enterprise edition

Yuheng's core — document parsing, retrieval and Q&A, collaborative documents, knowledge health — lives in this repository under the MIT license, and that stays true. The Enterprise edition adds a few optional capabilities on top of the community edition for larger organizations, the same open-core shape many open-source projects use: both editions share the same codebase and release line, and the Enterprise edition is not a separate product built from scratch.

## What exists today

The Enterprise edition has not shipped yet; everything below is planned, not available. A community deployment shows a matching card for each one under Settings → Enterprise, and none of them carries a purchase prompt — nothing is sold before it exists.

| Capability | What it does |
| --- | --- |
| Permission-aware retrieval | Access control at the level of individual entries: restricted knowledge is only ever answered to people who are allowed to see it |
| SAML single sign-on | Sign in through your identity provider, with a local emergency administrator login so a provider outage cannot lock you out |
| Audit log export | Export the audit log to files, or forward it to syslog or a webhook, for integration with an existing SIEM |
| Additional retrieval engines | Run retrieval on OpenSearch and other engines the community edition does not ship, alongside the built-in PostgreSQL (ParadeDB + pgvector) engine |

## How this is wired

The community edition has a dedicated seam for this (`internal/extension`): a `Features` registry that the core code only ever asks "is this on", never knowing or caring who implements it. The Enterprise edition attaches to that seam with a single blank import, without changing a line of the community code. The deployment's `GET /api/v1/system/capabilities` endpoint reports each Enterprise capability's state to the frontend; without the Enterprise edition installed, a capability is neither "locked" nor "enabled" — it simply does not exist, so the frontend shows a description only, with no lock icon or upgrade dialog.

Read [Extension points](../../06-development/03-extension-points.md) (Chinese) for the full design of this seam, or go straight to [`internal/extension` in the source](https://github.com/magicyuan876/Yuheng/tree/main/internal/extension).

## Get in touch

The Enterprise edition is early and has no sales channel yet. If one of the capabilities above would matter for your deployment, or you want to know where things stand, open a thread on [GitHub Discussions](https://github.com/magicyuan876/Yuheng/discussions) and say what you need.
