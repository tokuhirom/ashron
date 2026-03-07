# コンテキストエンジニアリング調査レポート

## 1. 現在の ashron の実装状況

### 1.1 コンテキスト管理フロー

```
User Input
  -> addUserMessage()
  -> NeedsCompaction() チェック
     -> true の場合:
        1. Prune() - 古いツール出力を200バイトに切り詰め
        2. Summarize() - LLMでサマリー生成
        3. BuildCompacted() - system messages + summary + 直近20メッセージで再構築
  -> stubOldToolResults() - 古いツール結果を "[stored: use get_tool_result...]" に置換
  -> SelectBuiltinTools() - プロンプトキーワードに基づくツール動的選択
  -> StreamChatCompletionWithTools() - API送信
  -> ツール実行
     -> ResultStore.Store(id, output) - フル結果を保存
     -> CompactToolResultForHistory() - 履歴用にコンパクト化
  -> continueConversation() - 次のターンへ
```

### 1.2 現在の設定

| 設定 | 説明 |
|------|------|
| `MaxMessages` | メッセージ数上限 |
| `MaxTokens` | トークン数上限 |
| `CompactionRatio` | compaction発動の閾値比率 |
| `AutoCompact` | 自動compactionの有効/無効 |

### 1.3 既存の最適化

- **ツール結果スタブ化**: 古いツール結果を `get_tool_result` で取得可能なスタブに置換（observation masking の一種）
- **ツール出力のコンパクト化**: ツール種別ごとに異なる上限（デフォルト4000B、read_file: 8000B、execute_command: 6000B）。超過時は head 75% + tail 25% で切り詰め
- **動的ツール選択**: プロンプトのキーワードに基づき必要なツールだけを選択し、ツール定義のトークンを削減
- **サブエージェント分離**: 各サブエージェントは独立したコンテキストウィンドウを持つ
- **永続メモリ**: global/project スコープの永続メモリでセッション間の情報保持

---

## 2. 最新のコンテキストエンジニアリング知見（2025-2026）

### 2.1 四つの基本戦略（LangChain分類）

| 戦略 | 説明 | ashron対応状況 |
|------|------|----------------|
| **Write** | スクラッチパッドや永続メモリに情報を書き出す | 部分的（memory_write あり） |
| **Select** | 関連情報を選択的に取得する | 部分的（get_tool_result あり） |
| **Compress** | サマリーやトリミングでトークンを削減する | あり（Prune + Summarize） |
| **Isolate** | サブエージェントやサンドボックスでコンテキストを分離 | あり（subagent） |

### 2.2 Observation Masking（JetBrains研究 / NeurIPS 2025）

**最も費用対効果の高い手法。ashron は既に部分的に実装済み。**

- ツール出力（observation）のみをマスクし、エージェントの推論と行動履歴は保持
- 固定ウィンドウ（最適値: 10ターン）の外側のobservationを `"details omitted for brevity"` に置換
- LLMサマリーより4/5のテスト構成で優れた結果
- **50%以上のコスト削減**、95%以上の精度を維持
- サマリーと比較して **キャッシュ再利用率が高い**（変更が少ないため prefix caching が効く）

**ashron の現状との差分:**
- ashron の `stubOldToolResults()` は全古いツール結果をスタブ化 → 近い
- ただし「直近Nターン」のウィンドウ制御がない（現在は最後のツール結果のみフル保持）
- Prune の切り詰めは200バイトで固定 → ツール種別による調整なし

### 2.3 Compaction vs Summarization vs Verbatim Compaction

| 手法 | 圧縮率 | 精度 | 検査可能性 | 幻覚リスク |
|------|--------|------|------------|------------|
| LLMサマリー | 80-90% | 3.74-4.04/5 | あり | 中 |
| 不透明圧縮 (OpenAI) | 99.3% | 3.43/5 | なし | 不明 |
| Verbatim Compaction | 50-70% | 98% | あり | なし |

**重要な発見: Re-reading Loop問題**
- サマリーが50Kトークンのgrep結果を2Kに圧縮 → エージェントが詳細を失い再検索 → コンテキストが再び満杯 → 進捗なし
- **対策**: コード編集エージェントにはverbatim compaction（原文のトークン削除）が適する

### 2.4 ハイブリッドアプローチ（推奨）

JetBrains の研究が提案する多層防御:

1. **第1層: Observation Masking**（常時・低コスト）
   - 古いツール出力をマスク、推論チェーンは保持
2. **第2層: Verbatim Compaction**（中間段階）
   - 低関連度のトークンを削除（原文保持）
3. **第3層: LLMサマリー**（最終手段）
   - コンテキストが依然大きい場合にのみ発動

### 2.5 Compaction発動タイミング

- **推奨閾値: 80%**（95%ではなく）。85%→80%で平均2.3秒のレスポンス改善（Anthropic changelog 2026年1月）
- 仮想ファイルスナップショットによるリカバリ機能を持たせると安全

### 2.6 Just-In-Time Context Retrieval（Anthropic推奨）

- 事前にすべてのデータをロードするのではなく、軽量な識別子（ファイルパス、URL）だけ保持
- 必要時にツールでデータを動的ロード
- **コンテキスト汚染の回避**: 無関係な事前ロードデータを排除
- ashron の `get_tool_result` は既にこのパターン

### 2.7 構造化ノートテイキング（Agentic Memory）

- エージェントがタスク中に定期的に永続ノートを書き出す
- コンテキストウィンドウ外に保存し、後で取得可能
- 長時間タスクの進捗追跡、重要な決定事項の保持
- ashron の `memory_write` は既にこれに対応

### 2.8 サブエージェントアーキテクチャ（Anthropic推奨）

- 各サブエージェントに数万トークンのコンテキストを使わせる
- **メインエージェントへは1,000-2,000トークンの要約を返す**
- 複雑な研究タスクでシングルエージェントを大幅に上回る

### 2.9 Context Rot（コンテキスト腐敗）

- コンテキストが長くなるほどリコール精度が低下する2026年の主要課題
- **対策**: コンテキストに入るすべての情報をキュレーション・構造化・検証する
- 「集中した300トークンのコンテキストが、散漫な113,000トークンのコンテキストに勝る」

### 2.10 AGENTS.md の効果

- 実証データ: ランタイム中央値 29% 削減、出力トークン 17% 削減
- 構造化プロンプトはナラティブプロンプトより **30% 少ないトークン** を消費
- 継続的な更新が重要

---

## 3. 改善提案（優先度順）

### 3.1 [高] Observation Masking の強化

**現状**: `stubOldToolResults()` が全古いツール結果をスタブ化するが、ウィンドウ制御なし。

**改善案**:
- 直近 N ターン（推奨: 10）のツール結果はフル保持
- N ターンより古いツール結果のみスタブ化
- ツールの呼び出し自体（名前・引数）は常に保持

```go
// stubOldToolResults を改善
// 直近 recentToolWindow ターンのツール結果はフル保持
const recentToolWindow = 10

func stubOldToolResults(messages []Message, recentWindow int) []Message {
    // 直近 recentWindow 個のツール結果を特定
    // それより古いもののみスタブ化
}
```

**期待効果**: キャッシュヒット率向上、コンテキスト品質改善

### 3.2 [高] Compaction 閾値の調整

**現状**: `CompactionRatio` で設定可能だが、デフォルト値の最適化が不明。

**改善案**:
- デフォルト閾値を **80%** に設定（現在値が高すぎる場合）
- compaction 前の PreCompact フック対応（重要情報の事前保存）

### 3.3 [高] 段階的コンテキスト管理

**現状**: 「全保持 → compaction」の二段階のみ。

**改善案**: 三段階の防御を実装

```
Stage 1: Observation Masking（常時）
  - 古いツール出力をスタブ化（get_tool_result で復元可能）

Stage 2: Pruning 強化（80% 到達時）
  - ツール種別に応じた intelligent truncation
  - 重要度ベースのメッセージ選別

Stage 3: LLM Summarization（90% 到達時）
  - 最終手段としてのフルサマリー
  - サマリープロンプトの最適化（アーキテクチャ決定・未解決バグ・実装詳細を優先保持）
```

### 3.4 [中] ツール種別に応じたスマート切り詰め

**現状**: `CompactToolResultForHistory()` でツール種別ごとの上限はあるが、切り詰め方法は一律（head 75% + tail 25%）。

**改善案**:
- `grep` / `glob` 結果: マッチ数のサマリー + 上位N件のみ保持
- `read_file`: ファイルパス + 行数 + 変更した部分のみ保持
- `execute_command`: exit code + stderr のみ保持（成功時）、フル出力（失敗時）
- `list_directory`: ディレクトリ構造のサマリーのみ

### 3.5 [中] Compaction サマリープロンプトの最適化

**改善案**: サマリー生成時のプロンプトに以下を明示的に指示:
- 変更したファイルとその内容を優先保持
- 未解決の問題・バグを保持
- アーキテクチャ上の決定事項を保持
- ユーザーの好みや指示を保持
- 中間的な探索結果は圧縮可

### 3.6 [中] サブエージェントの自動 Compaction

**現状**: サブエージェントは compaction なし。

**改善案**:
- サブエージェントにも `ContextConfig` を適用
- ただしサブエージェントは短命タスクが多いため、閾値を高めに設定

### 3.7 [低] コンテキスト使用量の可視化

**現状**: `CompactionStatus()` はあるがUIに未統合。

**改善案**:
- ステータスバーにトークン使用量 / 閾値を表示
- compaction 発生時にユーザーに通知

### 3.8 [低] スクラッチパッド機能

**現状**: `memory_write` はセッション横断の永続メモリ。

**改善案**:
- セッション内のみ有効な `scratchpad_write` / `scratchpad_read` ツールを追加
- compaction 時にスクラッチパッドの内容をコンテキストに再注入
- 長時間タスクの進捗追跡に有用

### 3.9 [低] Prefix Caching 最適化

**改善案**:
- system messages を安定させて prefix caching の効果を最大化
- compaction 後もシステムメッセージの順序と内容を一定に保つ
- observation masking はキャッシュフレンドリー（変更が先頭に集中しない）

---

## 4. 参考文献

- [Effective context engineering for AI agents - Anthropic](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
- [Cutting Through the Noise: Smarter Context Management for LLM-Powered Agents - JetBrains Research (NeurIPS 2025)](https://blog.jetbrains.com/research/2025/12/efficient-context-management/)
- [Context Engineering for Agents - LangChain](https://blog.langchain.com/context-engineering-for-agents/)
- [Context Engineering for Coding Agents - Martin Fowler](https://martinfowler.com/articles/exploring-gen-ai/context-engineering-coding-agents.html)
- [Compaction vs Summarization: Agent Context Management Compared - Morph](https://www.morphllm.com/compaction-vs-summarization)
- [Context compression - Google ADK](https://google.github.io/adk-docs/context/compaction/)
- [Context Engineering Best Practices for Agentic Systems - Comet](https://www.comet.com/site/blog/context-engineering/)
- [Context Engineering - LLM Memory and Retrieval for AI Agents - Weaviate](https://weaviate.io/blog/context-engineering)
- [Context Engineering Guide - Prompt Engineering Guide](https://www.promptingguide.ai/guides/context-engineering-guide)
