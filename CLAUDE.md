# CLAUDE.md

## コマンド

### テスト（Docker経由で実行すること）
```bash
docker compose run --rm backend-test
docker compose run --rm frontend-test
```

### ローカル開発
```bash
cd backend && go run .          # :8080
cd frontend && npm run dev      # :5173 (proxy → :8080)
```

### 本番相当
```bash
docker compose up --build -d    # :3000
```

## 設計判断

- 投票は1議題ずつではなく、**複数の質問に一括回答**する形式（意図的な設計）
- 投票の選択肢は**自由テキスト**（MVP方針）
- セッション履歴のレポート機能は**不要**
- ルームは**継続利用**前提（使い捨てではない）
- ユーザーアカウント機能は**導入予定なし**

## 進行中の作業

TODO.md を参照。
