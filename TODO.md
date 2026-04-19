# DB設計リファクタリング タスク一覧

## インメモリ移行

`participants` / `sessions` / `answers` はリアルタイム投票の一時データであり、DB永続化が不要。
Hub（WebSocket管理）のインメモリ状態に移行する。

### 対象ファイル
- `backend/internal/handler/ws.go` — Hub に状態管理を追加
- `backend/internal/handler/room.go` — Login/Validate を Hub 経由に
- `backend/internal/handler/session.go` — 全メソッドを Hub 経由に
- `backend/internal/handler/question.go` — answers FK クリーンアップ削除
- `backend/internal/db/db.go` — 3テーブル削除
- `backend/internal/model/model.go` — 型の整理
- `backend/main.go` — ハンドラ初期化の調整
- `backend/internal/handler/handler_test.go` — テスト修正

### タスク

- [x] `db.go`: participants/sessions/answers テーブルをスキーマから削除
- [x] `ws.go`: Hub に participants/sessions/answers のインメモリ状態管理を追加（ID生成はatomicカウンター）
- [x] `room.go`: Login → Hub にパーティシパント登録、ValidateParticipant → Hub から検索、GetParticipants → Hub から取得（DB依存を除去）
- [x] `session.go`: StartSession/SubmitAnswers/RevealResults/ResetSession/GetVoteStatus/GetResults を全て Hub 経由に変更
- [x] `question.go`: UpdateQuestionSet 内の `DELETE FROM answers WHERE question_id IN (...)` を削除
- [x] `main.go`: SessionHandler/RoomHandler の初期化から不要な DB 依存を除去（SessionHandlerはquestions参照のためDB継続保持）
- [x] `model.go`: 不要になった Answer/Session/Participant 型を整理（Hub 内の型に統合）
- [x] テストを新アーキテクチャに合わせて修正・全件パス確認（25テスト全PASS、SKIP=0）

## DB改善（残る rooms/question_sets/questions テーブル）

- [ ] マイグレーションファイルの分離: 現在 `db.go` にインラインで埋め込まれているスキーマ定義を外部の `.sql` ファイルに分離する
- [ ] マイグレーション戦略の導入: 現在 `CREATE TABLE IF NOT EXISTS` のみでスキーマ変更が反映されない。`schema_version` テーブル + ALTER文、または golang-migrate 等のツール導入を検討
- [ ] `rooms.name` に UNIQUE 制約を追加: 名前検索（`WHERE name = ?`）で同名ルームが存在すると意図しないルームに入る問題を防止
- [ ] 外部キーカラムにインデックスを追加: `question_sets.room_id`, `questions.question_set_id` に CREATE INDEX（SQLite は FK に自動インデックスを作成しない）
- [ ] インメモリ移行後に残る FK 関係の ON DELETE 整合性を再確認
