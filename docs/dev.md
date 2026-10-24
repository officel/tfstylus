# Developer

> [!IMPORTANT]
> tfstylus は terraform （tfファイル）を人間が読みやすくするするためのツールです。
> あまり業務貢献はしないと思います。お仕事で利用される場合はご留意ください。

- tfstylus は Go 言語で書かれています
- 特別なことは何もないので、clone して build すればいいはずです（Goのランタイムは必要です）

```sh
# build
go build -o tfstylus ./cmd/tfstylus

# test
go test ./...
```

## Docker / pre-commit

Docker デーモンと BuildKit が必要です。リポジトリルートでビルドします。push は不要です。

```sh
docker build --pull -t tfstylus:local .
docker run --rm tfstylus:local --version
docker run --rm --network=none --read-only --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --user "$(id -u):$(id -g)" \
  --mount "type=bind,src=$(pwd),dst=/src,readonly" \
  tfstylus:local plan --recursive
```

- マルチステージビルドで静的バイナリを生成し、最終イメージにはバイナリとライセンスだけを配置します。シェル・パッケージマネージャー・Go ツールチェーンは含みません。
- 既定の実行ユーザーは非 root の `65532:65532` です。上の例ではホストの UID/GID を使用します。ホスト側が root なら非 root の UID/GID を指定してください。
- `plan` の変更なし・変更あり・エラーの終了コードはそれぞれ `0`・`1`・`2` のままです。読み取り専用マウントは `plan` 用です。`apply` / `init` を使う場合は、対象の書き込み権限とマウント設定を明示的に変更してください。
- ビルダーは Go のパッチバージョンとマルチプラットフォームの digest で固定しています。Go の更新時には `go.mod` の要件を確認し、タグと digest を合わせて更新してください。`--pull` だけでは固定した digest は更新されません。
- `.dockerignore` は必要なソース・埋め込み設定・依存定義・ライセンスだけを許可します。ソースの配置や埋め込みファイルを追加した場合は、この許可リストも更新してください。
- ホストのホームディレクトリの設定は自動的には共有しません。設定はマウントするリポジトリ内に置くか、必要な設定ファイルだけを追加で読み取り専用マウントし、`--config` で指定してください。

pre-commit / prek の利用側では、次のフックを選択します。`rev` は Dockerfile を含むコミットの SHA またはリリースタグに置き換えてください。

```yaml
repos:
  - repo: https://github.com/officel/tfstylus
    rev: <commit-sha-or-tag>
    hooks:
      - id: tfstylus-docker
        args: [--recursive]
```

`language: docker` はフックのリポジトリからローカルでイメージをビルドします。`tfstylus` を PATH に配置しているため、pre-commit による entrypoint の上書きにも対応します。ファイル名を渡さず、マウントされた利用側リポジトリを検査します。

pre-commit の Docker 実行は上の例の `--network=none`・`--read-only` などを自動では適用せず、リポジトリも書き込み可能でマウントします。厳格な分離が必要な場合は、上の `docker run` を呼ぶローカルの `language: system` フックなどで制限を明示してください。

## Lint

```sh
golangci-lint config verify
golangci-lint run --fix=false
```

- `gosec` はテストも検査します。ただし、テスト内の fixture 読み書きに対する G304 は設定で除外します。
- `writeConfig` の G703 は、全呼び出し元がテスト用一時ディレクトリと固定の fixture 名を使うため、書き込み行だけ理由付きの `#nosec` で除外します。
- 本体の意図したパス操作と公開可能な初期設定の権限、テストの権限保持検証などは、理由付きの `#nosec` で該当箇所だけ除外します。
- 警告を消すために権限保持のテストを弱めたり、ルールを全体で無効にしたりしないでください。パスの入力元や設定内容を変更する場合は、除外理由も再確認してください。

> [!CAUTION]
> [prek](https://github.com/j178/prek) で Git のフックを管理しています see `.pre-commit-config.yaml`
> ローカルで実施しなくても構いませんが、PR 時には CI として実行されます。CIが通らないPRはマージできません。

---

> [!CAUTION]
> [task](https://github.com/go-task/task) でコマンドの省力化をしています see `Taskfile.yml`
> 見ればわかる程度のコマンドの羅列です。Windows では `hash` コマンド等動かないケースがあります。
