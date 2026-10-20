# リリース運用

## 現在の自動化

- [release workflow](../.github/workflows/release.yml) は `v*` タグの push で起動し、GoReleaser の `release --clean` を実行します。
- [GoReleaser 設定](../.goreleaser.yaml) は GitHub Release に Linux / macOS / Windows の各 amd64・arm64 向けアーカイブと `checksums.txt` を作成します。Windows は zip、その他は tar.gz です。
- 同設定の Homebrew Cask は `officel/homebrew-tap` の `Casks` に更新ブランチを作成し、`main` 向け pull request を作ります。`skip_upload` は無効（コメントアウト）なので、公開処理に含まれます。
- workflow は `HOMEBREW_TAP_GITHUB_TOKEN` secret を GoReleaser の `GITHUB_TOKEN` として渡します。値をログ・文書・コマンド履歴へ出さず、必要な書き込み権限があることだけを管理画面で確認します。

## タグを push する前

- 変更を `main` に取り込み、対象コミットの CI が成功していることを確認します。CI はテスト、`go vet`、build、起動確認を行います。
- バージョンタグが既存の tag / Release と重複していないこと、tag が意図した `main` のコミットを指していることを確認します。
- GoReleaser 設定変更時は `goreleaser check` を実行し、設定を検証します。実際の publish を伴う `goreleaser release` はローカルで実行しません。
- tap repository と workflow secret が利用可能であることを確認します。トークンをソースや設定ファイルへ直接記載しません。

## タグ push 後

1. tfstylus の Actions で該当タグの **Release** workflow が成功したことを確認します。失敗時は失敗した GoReleaser の段階とログを確認し、Release / asset / tap PR がすでに作成されていないか調べてから再実行方法を判断します。公開済み tag を force 更新・削除して復旧しません。
2. GitHub Release の tag / version と、6つの OS・architecture 向け archive および `checksums.txt` を確認します。
3. Homebrew tap に対する Cask 更新 pull request の version、download URL、checksum、対象バイナリが Release と一致することを確認し、問題がなければ merge します。
4. 実利用環境でインストールと `tfstylus --version` を確認します。Homebrew、Linux、Windows の少なくとも公開対象の導入経路を確認し、結果を記録します。
5. macOS のインストール時に警告・拒否が生じる場合は、署名・公証の方針と実際の挙動を調べます。未決定のまま Gatekeeper を回避する設定を追加しません。

## 実績と確認できていない範囲

2026-10-20 に読み取り専用で確認した GitHub 上の実績:

- [v0.1.10 Release](https://github.com/officel/tfstylus/releases/tag/v0.1.10) は 2026-10-19 に公開され、Release workflow は成功しました。6アーカイブと checksum が添付されています。
- [v0.1.10 tap 更新 PR](https://github.com/officel/homebrew-tap/pull/10) は merge 済みです。
- v0.1.9 の Release workflow と [tap 更新 PR](https://github.com/officel/homebrew-tap/pull/6) も成功・merge 済みです。
- v0.1.8 の Release workflow は GoReleaser 設定の `pull_request.body` が GoReleaser v2 で認識されないため失敗しました。修正後の v0.1.9 / v0.1.10 は成功しています。

この履歴で自動 Release 作成から tap 更新 PR の merge まで確認できました。一方、各 OS での実インストール・起動、Homebrew による利用者側の導入、macOS の署名・公証は、この確認では実施していません。GitHub 上で設定や PR を確認したことを、各環境でのインストール試験済みとは扱いません。
