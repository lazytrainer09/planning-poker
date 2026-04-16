# Planning Poker

チーム内スプリントプランニング用のプランニングポーカーWebアプリ。

複数の質問に対してメンバーがテキストで回答し、全員回答後に一斉公開して議論する形式。

## 機能

- **ルーム管理** — ルーム作成（合言葉認証）、参加者一覧
- **質問セット** — 作成・編集・削除。1セットに複数の質問を登録（例: 作業量 / 複雑さ / 不確実性）
- **投票** — 質問セットを選んで開始 → 全員回答 → 自動一斉公開 → 再投票 or 次の見積もりへ
- **リアルタイム同期** — WebSocketで入退室・回答状況・結果公開を即時反映

## 技術スタック

| レイヤー | 技術 |
|----------|------|
| バックエンド | Go |
| フロントエンド | React + TypeScript (Vite) |
| リアルタイム通信 | WebSocket |
| DB | SQLite |
| インフラ | Docker Compose (Nginx + Go) |

## 起動方法

```bash
docker compose up --build -d
```

- **http://localhost:3000** — アプリ（Nginx経由）

バックエンドはDockerネットワーク内部のみで公開され、ホストからは直接アクセスしません。

## 構成

```
frontend (Nginx :3000)        backend (Go :8080)
┌─────────────────────┐      ┌─────────────────────┐
│ React 静的ファイル    │      │ REST API (/api/*)   │
│                     │──────▶│ WebSocket (/ws)      │
│ /api/* → proxy      │      │ SQLite               │
│ /ws   → proxy       │      │                     │
└─────────────────────┘      └─────────────────────┘
```

## ディレクトリ構成

```
planning-poker/
├── docker-compose.yml
├── Dockerfile                 # バックエンド用
├── main.go                    # エントリポイント
├── internal/
│   ├── db/db.go               # SQLite スキーマ
│   ├── model/model.go         # データモデル
│   └── handler/
│       ├── room.go            # ルーム API
│       ├── question.go        # 質問セット CRUD
│       ├── session.go         # 投票セッション
│       └── ws.go              # WebSocket ハブ
└── frontend/
    ├── Dockerfile             # フロントエンド用
    ├── nginx.conf
    └── src/
        ├── api.ts / ws.ts     # API・WSクライアント
        └── pages/
            ├── TopPage.tsx          # ルーム作成・ログイン
            ├── RoomPage.tsx         # ルームトップ
            ├── QuestionSetEditor.tsx # 質問セット編集
            └── VotingPage.tsx       # 投票・結果表示
```

## 停止

```bash
docker compose down
```

SQLiteデータは `poker-data` ボリュームに永続化されます。

## Credits

このアプリケーションは [multi-agent-shogun](https://github.com/yohey-w/multi-agent-shogun) を利用して作成されました。
