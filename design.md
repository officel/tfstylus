# tfstylus 概要

- tfstylus は terraform のスタイルを統一することを目的としたツールです
- 鉄筆やけがき針のような硬質な素材で線を引くように、スタイルを揃えたいという願いが込められています
- 主に terraform のスタイルガイドを参考にスタイルをチェックして整えます
- [Style Guide - Configuration Language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/style)
- [Standard Module Structure | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/modules/develop/structure)
- ユーザープラクティスに基づいた恣意的なスタイル設定を取り込むことで、prettier 同様にスタイルに対する論争を終わらせます

## 判断の優先順位

```text
Terraform の意味を壊さない
  > コメントの意味・所属を壊さない
  > 変更量を小さくする
  > すべてを完全にソートする
```

完全な整列よりも、不確実な領域を変更しないことを優先します。
この文書は実装予定を含む設計です。利用可能な機能は [README.md](README.md) を参照してください。
「候補」「未決定」と記載した仕様は、対象機能の実装前に合意します。

# 動作に関する期待

## デフォルトの挙動

- 引数なしで実行された場合、ヘルプおよびバージョン情報を出力して終了します
- `--help` / `-h` は Usage を出力して終了します
- `--version` / `-v` はバージョン情報を出力して終了します

## 未知のコマンド

- 未知のコマンドを指定した場合、標準エラーへ `unknown command: <command>` を出力し、終了コード 1 で終了します
- `check` と `run` は既知のコマンドとして扱います

## -c/--config 引数（全サブコマンドで有効）

- `-c/--config string` 引数を渡した場合、`string` をHCL形式の設定ファイルとして読み込みます
- 設定引数はサブコマンドの前後どちらにも指定できます。値の欠落と複数指定はエラーです
- 空の設定ファイルを有効とし、未定義の属性・ブロックをエラーにします
- 明示したファイルの読み込みまたは解析に失敗した場合、エラーで終了し、自動探索へ切り替えません
- デフォルトの設定ファイル名は `.tfstylus.hcl` とし、以下の順序で検索して最初に見つかったものを使用します

  - `./.tfstylus.hcl` カレントディレクトリ
  - `./.config/.tfstylus.hcl` .config ディレクトリ
  - 見つかるまでこの順序で上位のディレクトリを検索します
- ファイルシステムのルートまで見つからない場合は、ユーザーホームディレクトリで同じ順序の候補を確認します。ホームでも見つからない場合は設定なしとして続行します
- 候補の検査ではシンボリックリンクを追跡し、リンク先が通常ファイルなら使用します
- 不在以外の候補検査エラー、通常ファイルでない候補、発見した設定の読み込み・解析エラーは返し、親の候補へフォールバックしません

## 対象ファイルの探索

- `check` / `run` は、指定がなければカレントディレクトリ直下を対象にします。引数なしのヘルプ・バージョン表示は変更しません
- `--chdir DIR` / `--chdir=DIR` で探索の起点を指定します。相対指定は実行開始時のカレントディレクトリを基準に解決します
- `--recursive` で起点と配下の各ディレクトリを探索します。`--chdir` と併用できます。`--recursive=true` / `--recursive=false` も指定できます
- オプションはサブコマンドの前後に指定できます。`--chdir` の空値・欠落・複数指定、無効な真偽値、未知のオプション、追加の位置引数はエラーです
- 再帰では隠しディレクトリ（`.terraform`、`.git` など）とディレクトリへのシンボリックリンクを辿りません。通常の子ディレクトリはモジュール参照の有無によらず探索し、`module` の source は解析・追跡しません
- 再帰の起点が `.` の場合、隠し名のカレントディレクトリも含めます。明示した隠し名の起点とディレクトリリンクの起点は再帰ではスキップします。非再帰では明示先の直下を読みます
- 小文字の `.tf` / `.tf.json` を列挙します。`override.tf`、`*_override.tf` と JSON 版も対象です。命名規則からの除外は探索からの除外ではありません
- `.` で始まるファイル、末尾が `~` のファイル、先頭と末尾が `#` のファイルを除外します。`.tfvars`、`.tfvars.json`、その他の拡張子は対象外です
- 通常ファイルへのシンボリックリンクはリンク側のパスで列挙します。ディレクトリ・特殊ファイルは対象外です。壊れた候補リンクや候補の検査失敗はエラーです
- ディレクトリごとに直下のファイルをまとめ、ディレクトリとファイルを辞書順で列挙します。子のファイルを親の処理単位へ混ぜず、内部のパスは絶対パスに揃えます
- 空ディレクトリも探索結果に含め、対象ファイルがないこと自体はエラーにしません。起点不在、起点が非ディレクトリ、読込・検査失敗では部分結果を使用しません
- 設定は各対象ディレクトリを起点に、既存の親・ホーム探索順で解決します。相対 `--config` は各対象ディレクトリ基準、絶対指定は共通ファイルとして使用します。対象ファイルがないディレクトリでも設定を検査します
- プロセスのカレントディレクトリは変更しません。ヘルプ・バージョン表示時は対象と設定の探索を行いません
- `check` は列挙したファイルを変更しません。`run` は事前検証後に変更対象だけを書き換えます。JSON の整形と通常の一覧出力は行いません

## check サブコマンド

- ファイルは変更しない
- 変更されるファイルと変更箇所をリストし、元入力上の行・列とルール名・理由を表示します
- 元構想の dry-run に相当する非破壊の事前確認を、このサブコマンドが担当します
- 書き込み機能より先に unified diff を提供する `check --diff` を実装済みとし、`--diff` と通常出力は両方で入力ファイルを変更しません
- 終了コードは変更なし `0`、変更予定あり `1`、構文エラー・設定エラー・安全上の理由で変換をスキップした場合 `2` です。整列済みは出力なし、スキップは警告として表示します

## run サブコマンド

- ファイルをチェックして変更します
- 書き込みはこのサブコマンドの明示指定時だけ行い、`check` と同じ変換候補を使用します
- 書き込みを始める前に、全対象の入力と変換候補を解析・検証します。失敗があれば一切書き込みません
- 変更対象は通常ファイルに限り、シンボリックリンクは拒否します。置換後もpermission modeを維持します
- 元ファイルと同じディレクトリの一時ファイルへ候補を書き、書き込み・同期・close後にatomic renameで置換します。失敗時は一時ファイルを削除し、対象ファイルを保持します
- 読み込み後の外部変更は、置換直前にfile identityと内容を再確認して検出した場合に中止します。再確認からrenameまでの競合は完全には防げません
- 複数ファイルは順次置換します。各置換はatomicですが複数ファイルの一括更新ではなく、途中のI/O失敗で先行ファイルが更新済みの場合があります
- 成功時は出力なし・終了コード0、探索・設定・解析・安全性・I/Oエラーは詳細を出力して終了コード2です
- 元構想の `--dry-run` / `--check` / `--write` / `--in-place` は別名として追加せず、既存の `check` / `run` を維持します

# 処理に関する期待

## 設定ファイル

- すべての変換処理は設定によりオンオフを可能にします

## ファイルの命名規則

- 公式のスタイルガイドは、通常用とモジュール用の2つのドキュメントがあります
- 通常用は若干初心者向け、モジュール用は高度な使い方として terraform module のリポジトリ用になっています
- tfstylus ではこれらを組み合わせた命名規則およびファイル名を採用します

  - `locals.tf`
  - `main.tf`
  - `outputs.tf`
  - `terraform.tf`
  - `variables.tf`
  - また `backend.tf` および `providers.tf` を使用しないように求めます（`terraform.tf` に統一する）
  - `override.tf` は設計思想によるものなので命名規則としては無視します（あってもなくてもよい）

## リソースの並べ替え

- リソースの並べ替えは実施しません
- 特定の順番に並んでいるべきかもしれませんが、すべてのケースを洗うことはできないからです
- リソースの並び順による可読性はコードを書く人の領分だということです

## プロパティの並べ替え

- `terraform fmt` よりも少しだけ強い強制力で書式を統一します
- ブロック毎に期待値が異なります

### terraform ブロック

- [Terraform block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/terraform)
- `terraform` ブロックは `terraform.tf` ファイルに統一して配置します
- [Terraform block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/terraform#complete-configuration)
- 基本的にはこの方針に従いますがいくつか異なる点があります

  - 先頭は `required_version` 次は `backend` block です
  - `required_providers.<provider>` のobject内キーは `source`、`version`、`configuration_aliases` の順です。各キーは任意で、省略されたキーは飛ばします
  - `cloud` の body は `organization`、`hostname`、`token`、`workspaces` の順です。これは tfstylus のスタイル規約であり、Terraform の必須順序ではありません
  - `cloud.workspaces` の body は `name`、`project`、`tags` の順です。`name` モードと `tags` モードは排他ですが、存在する属性だけをこの順で整列します

`cloud_order` と `cloud_workspaces_order` は独立した設定で、どちらも既定値を `true` とします。各設定を無効にすると、その規則は変更も警告も行いません。`cloud_order` は cloud body の既知属性と `workspaces` block の配置だけを、`cloud_workspaces_order` はその block 内の既知属性だけを扱います。式・文字列・コメントを含む元ソースチャンクを移動し、`tags` 値内部の要素順は変更しません。空の cloud、属性の省略、null/空値は環境補完される可能性があるため、それだけを理由にエラーにせず、プロセス環境も参照しません。

構造上の不正を修復しません。複数の `workspaces` block と、両方の値が静的に non-null と確認できる `name` / `tags` の競合は、対象ルールが有効なら属性順に関係なく警告します。null や未評価の値から競合を推測しません。未知要素、独立コメント、曖昧なチャンク境界は、並べ替えが必要な場合に該当 body を保持して警告します。通常の整列済み入力には警告しませんが、上記の静的な意味違反は例外です。

版差分の照合基準は Terraform `v1.14.0` とします。これは仕様比較用の固定タグで、tfstylus の最低対応版を意味しません。`cloud` は v1.1.0 に存在し、`workspaces.project` は v1.5.7 の schema にはなく v1.6.0 に存在します。Terraform v1.10.0 より前、または Terraform Enterprise v202411-1 より前では、key-value tags ではなく key-only tags を使う公式注記があります。tfstylus は実行時の Terraform/TFE 版やタグ形式の互換性を検査せず、値をそのまま保持します。

`required_providers` の整列はprovider要求object内だけを対象とする限定ルールです。設定 `required_providers_order` で独立して有効・無効を切り替え、既定値は `true` とします。provider要求の宣言順や `terraform` body 全体の順序は変更しません。

`provider_meta` は provider が定義する schema に依存するため、内容を解釈・整列しません。
terraform body の他要素を変換する場合も、各 body 内の `provider_meta` block の個数・ラベル・相対順と、block の元 byte を保持します。
候補がこの条件を満たさない場合は入力を保持します。provider_meta 自体の外側配置は定義せず、schema 検証や provider 取得も行いません。
Terraform v1.14.0 の公式資料では実験的機能とされ、0.13.0 以上が推奨されていますが、これは tfstylus の最低対応版を定めるものではありません。

複数行でキーが静的かつ一意、項目境界とコメント所属を一意に判定できるobjectだけを整列します。bare/quotedの静的キーおよびHCLで有効な `=` / `:` 区切りを受け入れ、キー表記・区切りtoken・値の元ソースは再生成しません。区切りのcommaを含む項目チャンクを移動し、その所属が曖昧ならobject全体を変更せず警告します。単一行object、未知・重複・動的キー、曖昧なコメントがあるobjectも変更せず警告します。

`configuration_aliases` は `aws.west` のようなprovider設定参照を含むリストとして扱い、通常の定数評価を試みません。式・値およびaliases配列内の順序を保持します。legacy文字列形式（例: `aws = "~> 5.0"`）は変更せず、object形式への移行もしません。この例外は汎用map/object内キー整列を許可するものではありません。

### provider ブロック

- [Provider block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/provider)

### リソースブロック

- [resource block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/resource)
- リソースブロック内のプロパティをプラクティスに従いつつ次のように並び替えます

  - `count` および `for_each` のメタ引数
  - `id`、`name`、およびこれらの prefix
  - リソース毎の必須パラメーター
  - 非ブロックパラメーター
  - ブロックパラメーター
  - タグ引数（`tag`,`tags`,`tag_name`等）
  - `lifecycle` ブロック
  - `depends_on` メタ引数

- 元構想の「通常属性をすべてアルファベット順」は、この優先グループを置き換えません
- グループ内の通常属性のアルファベット順は候補です。`id` / `name` の prefix の範囲、必須パラメーターの判定方法、同一グループの詳細順は未決定です
- リソース固有の意味的な順序を追加する場合は、明示的な設定とテストを必要とします。プロバイダー取得やスキーマ検証を暗黙に行いません

### variable ブロック

- [Variable block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/variable)
- 整列順は `description`、`deprecated`、`sensitive`、`nullable`、`ephemeral`、`const`、`type`、`default`、最後に `validation` ブロックとします。プラクティスに基づく重要度および情報量の順であり、Terraform の必須順序ではなく tfstylus のスタイル規則です。
- 複数の `validation` は相対順を保持して後置します。型式、default の object/list/tuple、validation 内部、heredoc、コメントは元 byte のまま移動し、補完・削除・評価・内部整列をしません。変数宣言間の順序も保持します。
- `variable_order` は既定値 `true`。無効時はこのルールによる変更・警告を出しません。
- 未知属性・未知 block・不正なラベル構造は variable 全体を保持して警告します。一行 body も保持して警告します。これらがなく整列済みなら、境界コメントがあっても無変更・無警告です。移動が必要でコメント所属や要素境界を確定できなければ variable 全体を保持して警告します。
- ラベル数と識別子、validation のラベルなし構造だけを静的に確認し、Terraform validator は再実装しません。属性の欠落、型・default の整合性、validation の条件や必須属性、予約名や module 横断の重複・override の意味は検証対象外です。HCL 構文エラーは従来どおり書き込み前に拒否します。
- 候補を再解析し、対象 body 内の許可順序差分、各要素と所属コメントの元 byte、validation の相対順、対象外 byte の保持を確認します。
- 2026-10-11 に現行公式資料（v1.16.x 表示）と [Terraform v1.14.0 の decoder/schema](https://github.com/hashicorp/terraform/blob/v1.14.0/internal/configs/named_values.go) を照合しました。同タグには `type`、`description`、`default`、`sensitive`、`nullable`、`ephemeral`、`validation` があり、`const` と `deprecated` はありません。
  現行資料は両方を掲載し、`ephemeral` は v1.10 以降、`deprecated` は v1.15 以降と明記しています。`const` の導入版は断定せず現行資料に基づいて名前を認識します。比較タグは最低対応版ではなく、入力の Terraform 版を推測した引数削除や対応版の保証は行いません。

### output ブロック

- [Output block reference for the Terraform configuration language | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/output)
- 整列順は、`description`、`deprecated`、`sensitive`、`ephemeral`、`type`、`value`、`precondition`、`depends_on`です。
- `value` は必須、他の採用要素は任意として扱います。属性の欠落やTerraformの版・module文脈に応じた制約は検証・補完しません。複数の `precondition` は相対順を維持し、内部を整列しません。
- `output_order` は既定値 `true`。無効時はoutputルールによる変更と警告を行いません。未知要素、不正ラベル、一行body、移動が必要な領域の曖昧なコメント所属を検出した場合、そのoutput全体を保持して警告します。整列済みで移動不要なら、曖昧な境界コメントだけを理由に警告しません。
- 2026-10-11 に現行公式資料（v1.16.x 表示）と Terraform v1.14.0 の `internal/configs/named_values.go` を照合しました。
- 現行資料は `type` を掲載し、`deprecated` は v1.15 以降、`ephemeral` は v1.10 以降と明記しています。両者はchild moduleのoutputに限られます。
- v1.14.0のdecoder/schemaは `description`、必須の `value`、`depends_on`、`sensitive`、`ephemeral` と `precondition` / `postcondition` blockを認識し、`postcondition` はエラーにします。同タグには `type` と `deprecated` がありません。`type` の導入版は確認できていません。
- v1.14.0は比較対象であり最低対応版ではありません。実行Terraform版やroot/child moduleを推測して引数を追加・削除せず、`postcondition` を含む未知・非対応要素があるoutputは整列が必要な場合に保持して警告します。
- ソーターが未定義の属性を追加したり、対応バージョンを推測して属性を削除したりすることはありません

### data ブロック

- [data block reference | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/block/data)
- 元構想の `count`、`for_each`、通常属性、ネストブロック、`lifecycle`、`depends_on` 順は候補です。既存のリソース優先グループを適用するかも含め、詳細順は未決定です

### ネストしたブロック

- 通常属性とブロックを別グループとし、通常属性の後にネストブロックを置く方針を基本とします。リソースでは既存のタグ優先グループも維持します
- `lifecycle` は通常のネストブロックより後、`depends_on` は最後に置きます
- ネストブロックもコメント付きチャンクとして扱い、移動と内部の整列を別の操作として判断します
- `dynamic`、プロバイダー固有、未知のブロック型を勝手に意味付けしません。同型の反復ブロックなど、順序が意味を持つ可能性があるものは相対順を維持します
- 安全に移動できないブロックをまたぐ並べ替えは行わず、その領域をスキップします

## トップレベルブロックの並べ替え候補

- トップレベルブロックの並べ替えは実施しません
- 適切なファイル分割による管理はユーザー側責務で、その並び順については関与しないこととします

# 安全なソース変換

## 解析と編集方式

Go の `github.com/hashicorp/hcl/v2` と `hclsyntax` を使います。
構文木と位置情報で属性・ブロックの種類、名前、範囲を特定し、コメントは字句情報と周辺の元ソースから判定します。
位置の byte offset と、行・列や Unicode の文字数を混同しません。

1. 元ファイルを byte 列として読み込み、保持します。
2. HCL として解析し、失敗したら書き込みません。
3. 対象 body の属性・ブロックの元ソース範囲を特定します。
4. コメント・空白・改行を含むチャンクを作成し、所属を確定できなければ対象領域をスキップして警告します。
5. 許可された規則にしたがってチャンクを並べ替え、対象 body の範囲だけを置換します。
6. 変換候補を再解析し、式・非対象要素・コメントの保持を検証します。
7. 検証済みの候補を事前確認・差分表示に渡し、明示的な書き込み時だけ適用します。

元の式、文字列、heredoc、ネストしたブロックの断片は再生成しません。
複数の範囲を編集する場合、重複する親・子の編集をそのまま適用しないようにします。
構文解析成功だけでは Terraform の意味やコメント所属の保持を証明できません。

`hclwrite` は必要な限定用途に留めます。次の方式を並べ替えに用いません。

- HCL 全体を `map[string]any` へデコードして再出力する。
- 正規表現だけでコメント・heredoc・ネスト構造を分解する。
- `RemoveAttribute` と `SetAttributeRaw`、または `Expr().BuildTokens(nil)` だけで元属性を再生成する。
- `NewEmptyFile` に整列済み要素だけを追加し、元の未構造化トークンを捨てる。

## コメントの分類と所属

最低限、次を区別します。

- ファイルヘッダーとフッター。
- 属性直前・ブロック直前のコメント。
- 行末コメントとブロック末尾コメント。
- 独立コメントとセクションコメント。
- heredoc・文字列・テンプレート内部のコメントに見える文字列。

属性の終端と同じ行にあるコメントは、その属性の行末コメントとして扱います。
属性やブロックの直前に空行なしで連続するコメント行は、所属が一意に確定できる場合だけ先行コメントとして付属させます。
複数行の連続コメントもまとめて扱い、本文・種別（`#`、`//`、`/* ... */`）・相対順を保持します。

要素との間に空行があるコメントは原則として独立コメントです。
ファイル先頭から続くコメント群が後続要素と空行で分離されていればヘッダーとして先頭に固定し、ファイル末尾のフッターも末尾に固定します。
ブロック末尾や閉じ波括弧付近のコメント、複数要素にまたがる `/* ... */` など、所属が曖昧なものを推測して直後の要素へ付属させません。

独立コメントをまたいで要素を移動しません。
初期の安全モードでは、所属を確定できない独立・セクションコメントを含む対象 body の並べ替えをスキップし、理由と位置を `path:line:column: warning: skipped: reason` 形式で警告します。
`check` はこのスキップを検出した場合でも処理を継続し、終了コード `2` で返します。`run` は同じ検証失敗を書き込み前の安全性チェックとして扱います。
コメントを固定した部分領域ソートは将来の候補であり、初期実装で暗黙に行いません。

heredoc、文字列リテラル、テンプレート内部の `#` や `//` を、外側の要素に付属するコメントとして走査しません。
式内部にある本物のコメントも式の元断片と一緒に保持し、外へ取り出しません。

## コメント付きチャンク

概念上、移動単位は次の情報を含みます。実装上の型をこの形に固定するものではありません。

```text
Chunk {
  leading_comments
  leading_whitespace
  body
  trailing_comment
  trailing_newline
}
```

`body` だけでなく、所属が確定したコメントと周辺の元ソースをまとめて移動します。
たとえば、次の `# AMI` は `ami` と、行末コメントは `instance_type` と一緒に扱います。

```hcl
# AMI
ami = var.ami

instance_type = "t3.micro" # 小さいインスタンス
```

body の境界・空行・インデント・最終改行も失わないよう範囲を割り当てます。
チャンク間の空白の割り当てが曖昧な場合は変換しません。
同じ設定で再解析・再分割しても、2回目以降に差分が発生してはなりません。

## 並べ替えないもの

- map / object リテラル内部のキー。
- list / tuple の要素と `for` 式の要素順。
- heredoc、文字列、テンプレート式の内部。
- `dynamic` ブロックの内部。
- プロバイダー固有など、順序が意味を持つ可能性のあるブロックの相対順。
- 明示的な除外指定の対象範囲。

未知の属性・構文は削除せず保持し、安全に変換できない領域は警告して変更しません。
map / object 内部の整列は明示的なオプションが必要です。
リスト整列を将来追加する場合、属性単位の allowlist と ignore 指定を必須にし、コメントとの関連付けなしに並べ替えません。

## ignore ディレクティブの候補

元構想の例を、直後の要素全体を除外する候補として記録します。

```hcl
# tf-sort: ignore
resource "example" "manual" {
  second = 2
  first  = 1
}
```

`sort: off` / `sort: on` のような範囲指定も候補です。
綴り、空行の扱い、適用範囲、ネスト、未対応ディレクティブの診断、すべての変換への適用方法は実装前に合意します。
構文はプロジェクト全体で統一し、文書とテストへ反映します。
文字列内の例や通常のコメントを、推測で除外指示と解釈しません。

# 変換実装の検証

変換機能を追加した段階で、次の順に検証します。文書だけの変更で Terraform を実行する必要はありません。

1. 入力と変換候補の HCL 構文解析、元の式・非対象要素の保持を確認します。
2. `terraform fmt -check` またはプロジェクトで定義したフォーマット検証を行います。
3. 利用可能かつ準備済みの環境では `terraform validate` を行います。
4. `git diff --check` を行います。fixture の出力も検査対象にします。
5. 同じ設定でソーターを再実行し、2回目の出力が byte 単位で同一であることを確認します。
6. コメント保持のテストを確認します。

Terraform コマンドは開発時の検証であり、ソーターの CLI から暗黙に実行しません。
利用できない検証は理由とともに記録し、検証のために `terraform init`、プロバイダー取得、`plan`、`apply` を暗黙に行いません。
Go のテスト・静的検査・ビルドと pre-commit は [AGENTS.md](AGENTS.md) に従います。

## コメント保持の fixture

機能の対象範囲に応じて、既存テストへ小さな入出力 fixture を段階的に追加します。

- `#`、`//`、単一行・複数行の `/* ... */`。
- 属性直前・行末・ブロック直前・ブロック末尾のコメント。
- 独立・セクション・ヘッダー・フッターのコメント。
- 空行で分離されたコメントと、空行なしの連続コメント。
- ネストしたブロック内のコメントと、同型の反復ブロック。
- heredoc 内の `#` / `//`、文字列・テンプレート内のコメントに見える文字列。
- map、list、`for` 式内部のコメント。
- 日本語・Unicode を含むコメント、改行形式、最終改行の有無。
- 曖昧な所属、ignore 指定、未対応構文の安全なスキップ。

各 fixture で、本文・種別・相対順・関連要素との関係が保持されていることを確認します。
コメントの存在数だけで安全性を判断せず、独立コメントが別要素へ付け替えられていないことも検証します。
式・heredoc・文字列・非対象断片は元の byte 列を保持し、順序依存の可能性があるブロックの相対順を変えません。
再解析の成功、解析・候補検証失敗時の非破壊性、冪等性を検証します。
`terraform fmt` に成功しただけでコメント保持が安全だとみなしません。
