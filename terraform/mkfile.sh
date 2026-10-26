#!/usr/bin/env bash

set -euo pipefail

# コードフェンスの開始行として認識する形式:
#   ```言語:ファイル名
# 言語名に空白やコロンは含めず、ファイル名はコロンの後ろ全体とする。
readonly OPEN_FENCE_RE='^```([^[:space:]:]+):(.+)$'
readonly CLOSE_FENCE_RE='^```[[:blank:]]*$'

usage() {
  printf '使い方: %s MARKDOWN_FILE\n' "${0##*/}" >&2
}

# 出力先を相対パスに限定し、親ディレクトリへの移動を防ぐ。
is_safe_relative_path() {
  local path=$1

  [[ -n $path && $path != /* && $path != */ ]] || return 1
  case "/$path/" in
    */../*) return 1 ;;
  esac
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

markdown_file=$1
if [[ ! -f $markdown_file || ! -r $markdown_file ]]; then
  printf '読み込み可能な Markdown ファイルではありません: %s\n' "$markdown_file" >&2
  exit 1
fi

# 出力先は、入力 Markdown ファイルが置かれているディレクトリを基準にする。
markdown_directory=${markdown_file%/*}
if [[ $markdown_directory == "$markdown_file" ]]; then
  markdown_directory=.
fi

# フェンスが最後まで閉じている場合だけファイルに反映できるよう、
# 内容はいったん一時ファイルに保存する。
temporary_file=$(mktemp)
trap 'rm -f -- "$temporary_file"' EXIT

in_code_fence=false
fence_path=

while IFS= read -r line || [[ -n $line ]]; do
  if [[ $in_code_fence == true ]]; then
    if [[ $line =~ $CLOSE_FENCE_RE ]]; then
      if ! is_safe_relative_path "$fence_path"; then
        printf '安全でない出力先です: %s\n' "$fence_path" >&2
        exit 1
      fi

      output_file="${markdown_directory}/${fence_path}"
      output_directory=${output_file%/*}
      if [[ $output_directory != "$output_file" ]]; then
        mkdir -p -- "$output_directory"
      fi
      cat -- "$temporary_file" > "$output_file"
      printf '生成: %s\n' "$output_file"
      in_code_fence=false
      fence_path=
    else
      printf '%s\n' "$line" >> "$temporary_file"
    fi
    continue
  fi

  if [[ $line =~ $OPEN_FENCE_RE ]]; then
    # BASH_REMATCH[1] は言語名、[2] は出力先のファイル名。
    fence_path=${BASH_REMATCH[2]}
    : > "$temporary_file"
    in_code_fence=true
  fi
done < "$markdown_file"
