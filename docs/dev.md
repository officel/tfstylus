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
