<p align="center">
  <picture>
    <img src="./docs/images/logo.png" alt="Yuheng Logo" height="120"/>
  </picture>
</p>
<p align="center">
    <a href="https://github.com/magicyuan876/yuheng/blob/main/LICENSE">
        <img src="https://img.shields.io/badge/License-MIT-ffffff?labelColor=d4eaf7&color=2e6cc4" alt="License">
    </a>
    <a href="./CHANGELOG.md">
        <img alt="バージョン" src="https://img.shields.io/badge/version-0.7.2-2e6cc4?labelColor=d4eaf7">
    </a>
</p>

<p align="center">
| <a href="./README.md"><b>English</b></a> | <a href="./README_CN.md"><b>简体中文</b></a> | <b>日本語</b> | <a href="./README_KO.md"><b>한국어</b></a> |
</p>

<p align="center">
  <h4 align="center">

  [プロジェクト紹介](#-プロジェクト紹介) • [アーキテクチャ設計](#️-アーキテクチャ設計) • [コア機能](#-コア機能) • [クイックスタート](#-クイックスタート) • [ドキュメント](#-ドキュメント) • [開発ガイド](#-開発ガイド)

  </h4>
</p>

# 💡 Yuheng — ドキュメントを「生きたナレッジ」へ：RAG・Agent 推論・自動 Wiki を統合した LLM ナレッジフレームワーク

## 📌 プロジェクト紹介

**Yuheng（ユーヘン）** は、大規模言語モデル（LLM）をベースとしたオープンソースのナレッジフレームワークで、エンタープライズ級の文書理解、セマンティック検索、自律推論シナリオ向けに設計されています。

本フレームワークは **3 つのコア能力** を中心に構築されています：日常的な検索に最適な **RAG ベースのクイック Q&A**、ナレッジ検索・MCP ツール・Web 検索を自律的にオーケストレーションし複雑なマルチステップタスクを処理する **ReAct Agent 推論**、そして Agent が生のドキュメントから相互リンクされた Markdown ナレッジベースとインタラクティブなナレッジグラフを自律生成・維持する全く新しい **Wiki モード**（手動編集・バージョン履歴・ワンクリックロールバック対応）。ナレッジの整備も細かく制御可能で、**ツリー型フォルダー**がアップロード時のディレクトリ構造を保持し、**チャンク編集とバージョン履歴**により検索チャンクをドキュメントと同様に編集・差分比較・ロールバックできます。さらに、多様なデータソース連携（Feishu ナレッジベース / Feishu クラウドドライブ / Notion / Yuque / RSS、随時拡充中）、**ウェブサイト埋め込み Widget** による外部サイトへのエージェント公開、プログラム連携向けの**スコープ付き API キーと Principal モデル**、ワークスペースごとの**マルチインスタンスストレージバックエンド**、20 以上の LLM プロバイダー統合、Langfuse による全体可観測性と**ランタイムタスクキューダッシュボード + Worker プール統治**、**エンタープライズ向けマルチテナント RBAC（4 階層ロールマトリクス + リソース所有権 + テナント監査ログ）**、完全セルフホスト可能なモジュラーアーキテクチャと組み合わせることで、Yuheng は散在する文書を「検索可能・推論可能・継続的に進化する」専用ナレッジ資産へと昇華させます。

Feishu、Notion、Yuqueなどの外部プラットフォームからのナレッジ自動同期（他のデータソースも順次対応中）に対応し、PDF、Word、画像、Excelなど10以上の文書フォーマットをサポート。WeChat Work、Feishu、Slack、TelegramなどのIMチャネルから直接Q&Aサービスを提供できます。モデル層ではOpenAI、DeepSeek、Qwen（Alibaba Cloud）、Zhipu、Hunyuan、Gemini、MiniMax、NVIDIA、Ollamaなど主要プロバイダーに対応。全プロセスをモジュラー設計し、大規模モデル、ベクトルデータベース、ストレージなどのコンポーネントを柔軟に差し替え可能。ローカルおよびプライベートクラウドデプロイに対応し、データは完全に自己管理可能です。さらにYuhengは **Langfuse** とシームレスに統合され、Agentの推論、トークン消費、パイプラインに対する包括的な可観測性（オブザーバビリティ）を提供します。

## 📱 機能デモ

<table>
  <tr>
    <td colspan="2" align="center"><b>💬 インテリジェント Q&A 対話</b><br/><img src="./docs/images/qa.png" alt="インテリジェント Q&A 対話" width="100%"></td>
  </tr>
  <tr>
    <td width="50%" align="center"><b>📖 Wiki ブラウザ</b><br/><img src="./docs/images/wiki-browser.png" alt="Wiki ブラウザ" width="100%"></td>
    <td width="50%" align="center"><b>🕸️ Wiki ナレッジグラフ</b><br/><img src="./docs/images/wiki-graph.png" alt="Wiki ナレッジグラフ" width="100%"></td>
  </tr>
  <tr>
    <td width="50%" align="center"><b>🕘 Wiki ページのバージョン履歴とロールバック</b><br/><img src="./docs/images/wiki-revision-history.png" alt="Wiki ページのバージョン履歴とロールバック" width="100%"></td>
    <td width="50%" align="center"><b>✂️ チャンク編集とバージョン履歴</b><br/><img src="./docs/images/kb-chunk-edit.png" alt="チャンク編集とバージョン履歴" width="100%"></td>
  </tr>
  <tr>
    <td width="50%" align="center"><b>📁 フォルダーツリーと一括操作</b><br/><img src="./docs/images/kb-document-list.png" alt="ナレッジベースのフォルダーツリーと一括操作" width="100%"></td>
    <td width="50%" align="center"><b>🤖 Agent モード · ツール呼び出しプロセス</b><br/><img src="./docs/images/agent-qa.png" alt="Agent モードツール呼び出しプロセス" width="100%"></td>
  </tr>
  <tr>
    <td colspan="2" align="center"><b>🔭 可観測性 · Langfuse Tracing</b><br/><img src="./docs/images/langfuse.png" alt="Langfuse Tracing" width="100%"></td>
  </tr>
</table>

## 🏗️ アーキテクチャ設計

![yuheng-architecture.png](./docs/images/architecture.png)

文書解析・ベクトル化・検索から大規模モデル推論まで、全パイプラインをモジュラー分離。各コンポーネントは柔軟に差し替え・拡張可能。ローカル / プライベートクラウドデプロイに対応し、データ完全自己管理、ゼロバリアの Web UI で即座に利用開始。


## 🧩 機能概要

**インテリジェント対話**

| 機能 | 詳細 |
|------|------|
| インテリジェント推論 | ReACT プログレッシブ・マルチステップ推論、ナレッジ検索・MCP ツール・Web 検索を自律的にオーケストレーション |
| クイック Q&A | ナレッジベースベースの RAG Q&A、迅速かつ正確な回答 |
| Wiki モード | Agent主導で生のドキュメントから構造化された相互リンク済みMarkdown Wikiページを自動生成・保守；ブラウザ内手動編集、ページのバージョン履歴、行単位差分とワンクリックロールバック |
| ツール呼び出し | 組み込みツール、MCP ツール（OAuth2 リモートサービス・会話中 OAuth 含む）、Web 検索；`@Skill / @MCP` メンションでターン単位に Agent ランタイムを範囲化 |
| 対話戦略 | オンライン Prompt 編集、検索閾値チューニング、マルチターン文脈認識、Agent 単位の引用出力トグル |
| 推奨質問 | ナレッジベースの内容に基づく質問の自動生成と回答後のフォローアップ |
| 一時添付 | セッションスコープで画像 / ドキュメントをアップロードし、非同期解析して一回限りの Q&A に使用（画像 + 添付の合算上限） |
| 引用と RAG 進捗 | インライン引用ポップオーバーと引用ドロワー（Web / KB ソースの区別）、統一 Markdown レンダリング、RAG パイプラインの段階別進捗表示 |
| セッション管理 | サイドバーでソース別（Web / IM / 埋め込み）にセッションをフィルター・グループ化、セッションタイトルのインラインリネーム対応 |

**ナレッジ管理**

| 機能 | 詳細 |
|------|------|
| ナレッジベースタイプ | FAQ / ドキュメント / Wiki、フォルダーインポート・URL インポート・複数タグ管理・オンライン入力 |
| フォルダーツリー | フォルダーアップロード時の元のディレクトリ構造を保持し、サイドバーのツリーで参照、フォルダーのリネーム、ドキュメントの別フォルダーへの再配置に対応 |
| チャンク編集とバージョン | UI から検索チャンクを直接編集、バージョン単位のスナップショット・差分・ワンクリックロールバック、編集後のインデックス自動再構築；生成質問の追加・編集・削除・再生成；ドキュメントのカスタムメタデータ対応 |
| アップロード単位の解析設定 | アップロード確認ダイアログまたは `process_config` API でパーサー・チャンキング・マルチモーダル（VLM / ASR）・グラフ抽出・質問生成をバッチ単位で上書き；reparse 時も設定変更可能 |
| 一括 reparse | 複数ドキュメントの解析を一度に再キュー、バッチ単位の `process_config` 対応 |
| データソースインポート | Feishu ナレッジベース / Feishu クラウドドライブ / Lark / Notion / Yuque / RSS フィードの自動同期（他のデータソースも開発中）、増分・全量同期対応 |
| 文書フォーマット | PDF / Word / Txt / Markdown / HTML / EPUB / MHTML / 画像 / CSV / Excel / PPT / JSON |
| 検索戦略 | BM25 疎検索 / Dense 密検索 / GraphRAG グラフ強化 / 親子チャンキング / pgvector HNSW 加速（1024 次元）/ 多次元インデックス |
| 一括選択とタグ付け | KB リストでマーキー（ドラッグ）複数選択し、一括 reparse と一括タグ付け（共通タグを自動プリセット）を実行 |
| E2E テスト | 検索+生成の全パイプライン可視化、リコール的中率・BLEU / ROUGE 指標評価 |

**連携と拡張**

| 機能 | 詳細 |
|------|------|
| 大規模モデル | OpenAI / Azure OpenAI / Anthropic (Claude) / DeepSeek / Qwen (Alibaba Cloud) / Zhipu / Hunyuan / Doubao (Volcengine) / Gemini / MiniMax / NVIDIA / Novita AI / SiliconFlow / OpenRouter / Requesty / Ollama |
| Embedding | Ollama / BGE / GTE / OpenAI 互換 API |
| ベクトル DB | PostgreSQL (pgvector) / Elasticsearch / OpenSearch / Milvus / Weaviate / Qdrant / Apache Doris / Tencent VectorDB |
| オブジェクトストレージ | ローカル / MinIO / AWS S3（IAM Role / IRSA のデフォルト資格情報チェーン対応）/ 火山引擎 TOS / Alibaba Cloud OSS / 金山雲 KS3 / 華為雲 OBS；**ワークスペースごとに複数のストレージインスタンス**、KB 単位のバインドとデフォルトインスタンス |
| IM 統合 | WeChat Work / Feishu / Lark（Feishu 国際版）/ QQBot / Slack / Telegram / DingTalk / Mattermost / WeChat / Yunzhijia |
| ウェブ埋め込み | 埋め込み Widget でエージェントを公開、ドメイン許可リスト・レート制限・セキュアモード Token 交換 |
| Web 検索 | DuckDuckGo / Bing / Google / Tavily / Baidu / Ollama / SearXNG / Keenable / Zhipu AI |
| API 連携 | スコープ付き API キー（能力単位の付与 + KB 単位の制限 + 節流付き last_used 追跡）と API 連携プレイグラウンド；MCP OAuth と埋め込みセッションを Principal 単位で分離；`resource_urls=public` で直接読み込み可能なファイル / 画像 URL を返却し、認証プロキシへの二次リクエストを不要に |
| MCP Server | 公式 PyPI パッケージ `yuheng-mcp`、29 ツール、stdio / SSE / HTTP の 3 トランスポート対応 |

**プラットフォーム**

| 機能 | 詳細 |
|------|------|
| デプロイ | ローカル / Docker / Kubernetes (Helm)、プライベート化・オフラインデプロイ対応 |
| UI | Web UI / RESTful API / CLI (`yuheng`) / Chrome Extension / ウェブ埋め込み Widget / WeChat ミニプログラム |
| 可観測性 | Langfuse（唯一のトレーシングバックエンド）で ReAct ループ・トークン消費・ツール呼び出し・パイプライン追跡；Langfuse 風のドキュメント解析トレースタイムラインを内蔵し、ステージごとの進捗を表示；システム管理者向けランタイムタスクキューダッシュボード（キュー深度・モデル別並行度・失敗タスクの調査と手動再試行） |
| タスク管理 | MQ 非同期タスク、ステージ別 Worker プール統治（core / 後処理 / enrichment / maintenance + 弾性共有プール、Wiki は独立プール）とモデル別バックグラウンド並行度ガバナー；バージョンアップ時の DB 自動マイグレーション |
| モデル管理 | 集中設定、YAML 宣言型ビルトインモデル設定、ナレッジベース単位のモデル選択、モデル単位の思考モード・Embedding 次元上書き、インタラクティブモデルデバッガー、マルチテナント組み込みモデル共有、集中管理される共有インフラ |

## 🐋 DeepSeek Harness プラグイン

[**`@magicyuan876/dsh-yuheng`**](./packages/dsh-yuheng/README.md) は公式の [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)（`dsh`）プラグインです。harness 自体は検索・埋め込み・ナレッジベースの機能を持たないため、このプラグインがコーディングエージェントに自社ドキュメントを与えます。`dsh plugin --profile web add @magicyuan876/dsh-yuheng` でインストールしてデプロイ先を指定すると、4 つの読み取り専用ツールがエージェントのツールセットに現れます。

- **`yuheng_search`** — ハイブリッド検索。原文のパッセージをそのまま返し、各件に再利用可能な `knowledge_id` が付く
- **`yuheng_read_document`** — 1 つのドキュメントのチャンクを順番に再構成、ページング対応
- **`yuheng_ask`** — Yuheng 自身が引用付きで作成した回答（RAG または ReAct パイプライン）
- **`yuheng_list_knowledge_bases`** — ナレッジベースの名前と id。エージェントが自分で検索範囲を絞れる


## 🚀 クイックスタート

### 🛠 環境要件

- [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)
- [Git](https://git-scm.com/)

### 📦 インストール・起動

```bash
git clone https://github.com/magicyuan876/yuheng.git
cd yuheng
cp .env.example .env   # 必要に応じて .env を編集（詳細はファイル内のコメント参照）
docker compose pull     # 最新イメージを取得
docker compose up -d    # コアサービスを起動
```

起動後、**http://localhost** にアクセスして利用開始。

> ローカル Ollama モデルを使用する場合は、先に `ollama serve > /dev/null 2>&1 &` を実行してください。

### 🔄 アップグレード

既存のデプロイがあり、新しい release をダウンロードした場合：

```bash
# .env の YUHENG_VERSION を対象バージョン（例: 0.7.0）に設定、または latest のまま
docker compose pull     # YUHENG_VERSION に一致するイメージを取得
docker compose up -d    # 新しいイメージでコンテナを再作成
```

> `docker compose up -d` のみではローカルキャッシュのイメージが再利用され、Web UI の表示バージョンがダウンロードした release と一致しない場合があります。

### 🔧 オプションサービス（Docker Compose Profile）

`--profile` フラグで追加コンポーネントを有効化。複数の profile を組み合わせ可能：

| Profile | 説明 | コマンド |
|---------|------|---------|
| _(デフォルト)_ | コアサービス | `docker compose pull && docker compose up -d` |
| `full` | 全機能 | `docker compose --profile full pull && docker compose --profile full up -d` |
| `neo4j` | ナレッジグラフ (Neo4j) | `docker compose --profile neo4j pull && docker compose --profile neo4j up -d` |
| `minio` | オブジェクトストレージ (MinIO) | `docker compose --profile minio pull && docker compose --profile minio up -d` |
| `langfuse` | トレーシング (Langfuse) | `docker compose --profile langfuse pull && docker compose --profile langfuse up -d` |

組み合わせ例：`docker compose --profile neo4j --profile minio pull && docker compose --profile neo4j --profile minio up -d`

サービス停止：`docker compose down`

### 🌐 サービスアドレス

| サービス | URL |
|---------|-----|
| Web UI | `http://localhost` |
| バックエンド API | `http://localhost:8080` |
| Langfuse トレーシング | `http://localhost:3000` |

## 文書ナレッジグラフ

Yuhengは文書をナレッジグラフに変換し、文書内の異なる段落間の関連関係を表示することをサポートします。ナレッジグラフ機能を有効にすると、システムは文書内部の意味関連ネットワークを分析・構築し、ユーザーが文書内容を理解するのを助けるだけでなく、インデックスと検索に構造化サポートを提供し、検索結果の関連性と幅を向上させます。

詳細な設定については、[ナレッジグラフ設定ガイド](./docs/KnowledgeGraph.md)をご参照ください。

## 対応するMCPサーバー  

[MCP設定ガイド](./mcp-server/MCP_CONFIG.md) をご参照のうえ、必要な設定を行ってください。


## 📘 ドキュメント

**公式製品ドキュメント**：[`website-docs/`](./website-docs/README.md) — 「入門 → アーキテクチャ → 機能 → API → クライアント → 開発」の 6 セクションで構成された完全なドキュメントセット。約 360 の API エンドポイント、約 150 の環境変数、9 つの拡張ポイントを網羅しています。このディレクトリは VitePress サイトでもあり、`cd website-docs && npm install && npm run dev` でローカルプレビュー、同ディレクトリの `Dockerfile` で単独デプロイも可能です。

よくある問題の解決：[よくある問題](./docs/QA.md)

詳細なAPIドキュメントは：[APIドキュメント](./docs/api/README.md)を参照してください

製品計画と今後の機能：[Roadmap](./docs/ROADMAP.md)

## 🧭 開発ガイド

### ⚡ 高速開発モード（推奨）

コードを頻繁に変更する必要がある場合、**Dockerイメージを毎回再構築する必要はありません**！高速開発モードを使用してください：

```bash
# インフラストラクチャを起動
make dev-start

# バックエンドを起動（新しいターミナル）
make dev-app

# フロントエンドを起動（新しいターミナル）
make dev-frontend
```

**開発の利点：**
- ✅ フロントエンドの変更は自動ホットリロード（再起動不要）
- ✅ バックエンドの変更は高速再起動（5-10秒、Airホットリロードをサポート）
- ✅ Dockerイメージを再構築する必要がない
- ✅ IDEブレークポイントデバッグをサポート

**詳細ドキュメント：** [開発環境クイックスタート](./docs/开发指南.md)

## 🤝 貢献ガイド

[Issue](https://github.com/magicyuan876/yuheng/issues) や Pull Request の提出を歓迎します。

**フロー：** Fork → ブランチ作成 → 変更をコミット → PR を作成

**規約：** `gofmt` でコードをフォーマット、[Conventional Commits](https://www.conventionalcommits.org/) に従う（`feat:` / `fix:` / `docs:` / `test:` / `refactor:`）

## 🔒 セキュリティ通知

**重要：** v0.1.3バージョンより、Yuhengにはシステムセキュリティを強化するためのログイン認証機能が含まれています。v0.2.0では、さらに多くの機能強化と改善が追加されました。本番環境でのデプロイメントにおいて、以下を強く推奨します：

- Yuhengサービスはパブリックインターネットではなく、内部/プライベートネットワーク環境にデプロイしてください
- 重要な情報漏洩を防ぐため、サービスを直接パブリックネットワークに公開することは避けてください
- デプロイメント環境に適切なファイアウォールルールとアクセス制御を設定してください
- セキュリティパッチと改善のため、定期的に最新バージョンに更新してください

## 👥 コントリビューター

素晴らしいコントリビューターに感謝します：

[![Contributors](https://contrib.rocks/image?repo=magicyuan876/yuheng)](https://github.com/magicyuan876/yuheng/graphs/contributors)

## 🙏 帰属表示

Yuheng にはサードパーティのオープンソースソフトウェアが含まれており、その相当部分は MIT
ライセンスで配布されている上流プロジェクト由来です。再配布時に同梱が必要な著作権表示と
ライセンス条項は次のファイルにあります:

- [`LICENSE`](./LICENSE) — ライセンス本文と元の著作権表示（無改変）。
- [`NOTICE`](./NOTICE) — 再配布時に必ず同梱すべき著作権行。
- [`THIRD_PARTY_NOTICES.md`](./THIRD_PARTY_NOTICES.md) と [`licenses/`](./licenses/) —
  依存関係ごとのライセンス全文と監査結果。

Yuheng は独立したプロジェクトであり、取り込んだソフトウェアの作者と提携・推奨・支援関係は
ありません。不具合報告とサポートは
[本リポジトリの Issues](https://github.com/magicyuan876/Yuheng/issues) へお願いします。

## 📄 ライセンス

このプロジェクトは [MIT](./LICENSE) ライセンスの下で公開されています。著作権表示と許諾表示を
同梱する限り、自由に使用・変更・配布できます。複製すべき具体的な行は [`NOTICE`](./NOTICE) を
参照してください。

## 📈 プロジェクト統計

<a href="https://www.star-history.com/#magicyuan876/yuheng&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=magicyuan876/yuheng&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=magicyuan876/yuheng&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=magicyuan876/yuheng&type=date&legend=top-left" />
 </picture>
</a>
