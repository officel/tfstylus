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

> [CAUTION]
> [prek](https://github.com/j178/prek) で Git のフックを管理しています see `.pre-commit-config.yaml`
> ローカルで実施しなくても構いませんが、PR 時には CI として実行されます。CIが通らないPRはマージできません。

---

> [CAUTION]
> [task](https://github.com/go-task/task) でコマンドの省力化をしています see `Taskfile.yml`
> 見ればわかる程度のコマンドの羅列です。Windows では `hash` コマンド等動かないケースがあります。
