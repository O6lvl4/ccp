# ccp — Claude Code Profile

AWS プロファイルのように Claude Code の設定を切り替える — 共有設定を複製せずに。

[English](./README.md)

## 背景：`CLAUDE_CONFIG_DIR` の問題

Claude Code は設定を `~/.claude/` から読み込みます。環境変数 `CLAUDE_CONFIG_DIR` を設定すれば別のディレクトリを参照させることができるため、用途別（仕事用・個人用・OSS用など）に設定を分けることが可能です。

しかし、ディレクトリを丸ごと切り替えると **すべてが別管理** になります：

| 設定ファイル | 内容 | 全部別管理になる |
|---|---|:---:|
| `CLAUDE.md` | グローバルな指示・ルール | yes |
| `settings.json` | 権限・hooks・プラグイン設定 | yes |
| `settings.local.json` | ローカル固有設定 | yes |
| `keybindings.json` | キーバインド | yes |
| `projects/` | プロジェクト別の memory・設定 | yes |
| `hooks/` | hook スクリプト | yes |

つまり、キーバインドやプロジェクトメモリなど**プロファイル間で共通にしたい設定まで個別に維持する必要がある**のが問題です。

## ccp の解決策：symlink overlay

ccp はプロファイル作成時に `~/.claude/` の全エントリを **symlink** としてプロファイルディレクトリに配置します。

```
~/.claude/                          ← ベース（常にそのまま使える）
~/.claude-profiles/work/
  CLAUDE.md          → ~/.claude/CLAUDE.md         ← 共有 (symlink)
  settings.json      (実ファイル)                    ← 上書き済み
  keybindings.json   → ~/.claude/keybindings.json   ← 共有 (symlink)
  projects/          → ~/.claude/projects/           ← 共有 (symlink)
```

### ポイント

- **デフォルトは全共有** — プロファイルを作った直後は `~/.claude/` と完全に同じ状態
- **変えたいファイルだけ分離** — `ccp override` で symlink を実ファイルに昇格
- **いつでも共有に戻せる** — `ccp share` で symlink に戻す
- **元の設定は一切変更されない** — `~/.claude/` には触れない。プロファイルを使わなければ今まで通り

## インストール

```bash
go install github.com/O6lvl4/ccp@latest
```

### シェル統合

`.zshrc` または `.bashrc` に以下を追加：

```bash
eval "$(ccp shell-init)"
```

これにより `ccp switch` 実行時に `CLAUDE_CONFIG_DIR` が自動で export されます。シェル関数で `switch` コマンドをラップする仕組みです。

> **なぜシェル統合が必要？**
> 子プロセス（Go バイナリ）から親シェルの環境変数は変更できないため、シェル関数経由で `eval` する必要があります。これは `nvm` や `pyenv` と同じアプローチです。

## 使い方

### プロファイルの作成と切り替え

```bash
# プロファイルを作成（~/.claude の全エントリを symlink）
ccp init work
ccp init personal

# プロファイルに切り替え
ccp switch work

# デフォルト（~/.claude）に戻す
ccp switch

# プロファイル一覧（* がアクティブ）
ccp list
# * work
#   personal
```

### 設定の分離と共有

```bash
# settings.json をこのプロファイル専用にする（実ファイルにコピー）
ccp override work settings.json

# CLAUDE.md も分離
ccp override work CLAUDE.md

# やっぱり CLAUDE.md は共有に戻す（symlink に復元）
ccp share work CLAUDE.md
```

### 状態確認

```bash
ccp status work
# profile: work
#
#   CLAUDE.md                      shared
#   settings.json                  overridden
#   keybindings.json               shared
#   projects/                      shared
#   hooks/                         shared
```

`shared` = symlink（ベースと同じ）、`overridden` = 実ファイル（プロファイル固有）

### ベースに追加されたファイルの同期

```bash
# ~/.claude に新しいファイルが増えた場合、symlink を追加
ccp sync work
#   + new-file.json
```

### プロファイルの削除

```bash
ccp delete work
```

## コマンド一覧

| コマンド | エイリアス | 説明 |
|---|---|---|
| `ccp init <name>` | — | プロファイルを作成 |
| `ccp switch <name>` | `sw` | プロファイルに切り替え |
| `ccp switch` | `sw` | デフォルト（`~/.claude`）に戻す |
| `ccp list` | `ls` | プロファイル一覧 |
| `ccp status [name]` | `st` | ファイルの shared/overridden 状態を表示 |
| `ccp override <profile> <file>` | `ov` | ファイルをプロファイル固有にする |
| `ccp share <profile> <file>` | `sh` | ファイルを共有（symlink）に戻す |
| `ccp sync <profile>` | — | ベースの新規ファイルを symlink で追加 |
| `ccp delete <name>` | `rm` | プロファイルを削除 |
| `ccp env` | — | 現在のプロファイルの export 文を出力 |
| `ccp shell-init` | — | シェル統合用の関数を出力 |

## 仕組み

```
ccp init work
```

1. `~/.claude-profiles/work/` を作成
2. `~/.claude/` 内の全エントリ（ファイル・ディレクトリ）に対して symlink を作成
3. この時点でプロファイルはベースと完全に同一

```
ccp switch work
```

1. `~/.claude-profiles/.active` にプロファイル名を書き込み
2. シェル関数が `ccp env` を eval → `export CLAUDE_CONFIG_DIR=~/.claude-profiles/work`
3. 以降 Claude Code はこのディレクトリを設定として使用

```
ccp override work settings.json
```

1. symlink のリンク先を読み取り
2. symlink を削除
3. リンク先の内容を実ファイルとしてコピー（ディレクトリの場合は再帰コピー）

```
ccp share work settings.json
```

1. 実ファイルを削除
2. `~/.claude/settings.json` への symlink を再作成

## ユースケース例

### 仕事用と個人用で権限を分ける

```bash
ccp init work
ccp override work settings.json
# work/settings.json を編集：厳格な権限設定に
# CLAUDE.md、keybindings、project memory は共有のまま
```

### OSS 活動用に CLAUDE.md を分ける

```bash
ccp init oss
ccp override oss CLAUDE.md
# oss/CLAUDE.md を編集：OSS 向けのルール・コンテキストを追加
# 権限やキーバインドは共有のまま
```

### チーム共有の設定を使う

```bash
ccp init team-project
ccp override team-project CLAUDE.md
ccp override team-project settings.json
# チーム固有の CLAUDE.md と settings.json を配置
# 個人の keybindings や project memory は共有のまま
```

## 要件

- Go 1.22+
- macOS または Linux
- `CLAUDE_CONFIG_DIR` をサポートする Claude Code
