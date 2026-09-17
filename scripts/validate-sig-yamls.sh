#!/usr/bin/env bash
# validate-sig-yamls.sh
# 校验 spec/governance/sigs/*.yaml 文件的格式与必需字段
#
# 用法：
#   ./scripts/validate-sig-yamls.sh [sigs_dir]
#
# 默认 sigs_dir 为 spec/governance/sigs
#
# 退出码：
#   0 - 全部通过
#   1 - 任一文件校验失败
#
# 校验覆盖范围（对应 spec/governance/sig-governance.md §5.2/§5.3）：
#   本脚本只覆盖"机器可判的必须项"，包括：
#     - 必需字段存在性与非空、status 枚举、labels 含 sig/<name>（labels 段内精确匹配）
#     - name 的 kebab-case 格式
#     - created_at / agents[].since 的 YYYY-MM-DD 日期格式
#     - leads / maintainers 非空且条目带 @
#     - 豁免注释双向一致性（leads 与 maintainers 为同一单人列表 <=> 顶部含 "# note: 单人维护豁免期内" 前缀注释）
#     - agents 每个条目均含 id / operator / since，id 为 kebab-case，operator 带 @
#     - scope_labels（可选字段）存在时条目必须为 scope/<kebab> 形式；跨文件重复映射输出 WARN
#
# 已知缺口（脚本不校验，由 PR review 人工兜底，见 sig-governance.md §5.3）：
#     - 仅支持 YAML block 风格列表（如 "leads: [\"@a\"]" 的 flow 风格不支持，会误报）
#     - 仓库人类 maintainer ≥ 2 时 leads/maintainers 单人重叠属 §4.1 违规而非 §4.2 豁免场景，
#       脚本会引导添加 §4.2 下无效的豁免注（必要条件反转，人工兜底）
#     - agents[].scopes 是否为 scope 字段的子集
#     - agents[].operator 是否在本 YAML 的 leads/maintainers 列表中（更严格的"必须是仓库 maintainer"无注册表可查）
#     - scope 路径是否真实存在于仓库
#     - scope_labels 映射的 label 是否真实存在于远端仓库 label 列表
#     - leads/maintainers 条目非空值（仅检查列表非空与 @ 前缀）
#     - CRLF 行尾文件对标量字段产生误报（pre-commit mixed-line-ending 兜底）

set -euo pipefail
shopt -s nullglob

SIGS_DIR="${1:-spec/governance/sigs}"

if [[ ! -d "$SIGS_DIR" ]]; then
  echo "[validate-sig-yamls] ERROR: directory not found: $SIGS_DIR" >&2
  exit 1
fi

# 必需字段（顶级）
REQUIRED_FIELDS=(
  name
  display_name
  status
  created_at
  leads
  maintainers
  scope
  labels
)

# 允许的 status 值
ALLOWED_STATUS="incubating|active|degraded|archived"

# kebab-case（不允许首尾连字符、不允许连续连字符）
KEBAB_CASE_RE='^[a-z][a-z0-9]*(-[a-z0-9]+)*$'

fail_count=0
file_count=0
# 跨文件收集 scope_labels 映射，用于查重
all_scope_labels=""

# 提取顶级标量字段值（容错：字段缺失时输出空串，不中断脚本；剥离行内注释与尾随空白）
get_field() {
  local field="$1" file="$2"
  grep -E "^${field}:" "$file" | head -n1 | sed -E "s/^${field}:[[:space:]]*//; s/[[:space:]]+#.*$//; s/[[:space:]]+$//" | tr -d '"'"'"'' || true
}

# 提取某个 block 列表字段的条目（每行一个，去引号、剥离行内注释与尾随空白；容错：字段缺失时输出空串）
get_list_entries() {
  local field="$1" file="$2"
  awk -v f="^${field}:" '
    $0 ~ f { in_section=1; next }
    /^[a-zA-Z_]+:/ && in_section { in_section=0 }
    in_section && /^[[:space:]]*-/ { print }
  ' "$file" | sed -E 's/^[[:space:]]*-[[:space:]]*//; s/[[:space:]]+#.*$//; s/[[:space:]]+$//; s/["'"'"']//g' || true
}

err() {
  echo "  ERROR: [$yaml_file] $*" >&2
}

for yaml_file in "$SIGS_DIR"/*.yaml; do
  # 跳过 schema 等下划线开头文件
  base="$(basename "$yaml_file")"
  if [[ "$base" == _* ]]; then
    continue
  fi

  file_count=$((file_count + 1))
  echo "[validate-sig-yamls] checking $yaml_file"

  # 检查必需字段
  for field in "${REQUIRED_FIELDS[@]}"; do
    if ! grep -qE "^${field}:" "$yaml_file"; then
      err "missing required field: $field"
      fail_count=$((fail_count + 1))
    fi
  done

  # 必需标量字段不得为空值（如 "status:" 后无值）
  for field in name display_name status created_at; do
    if grep -qE "^${field}:" "$yaml_file" && [[ -z "$(get_field "$field" "$yaml_file")" ]]; then
      err "required field must not be empty: $field"
      fail_count=$((fail_count + 1))
    fi
  done

  # 检查 status 值
  status_value="$(get_field status "$yaml_file")"
  if [[ -n "$status_value" ]] && ! [[ "$status_value" =~ ^($ALLOWED_STATUS)$ ]]; then
    err "invalid status value: $status_value (allowed: $ALLOWED_STATUS)"
    fail_count=$((fail_count + 1))
  fi

  # 检查 labels 中是否包含 sig/<name>（labels 段内精确匹配，防 sig/<name>-xx 绕过与 charter 等块内容误匹配）
  name_value="$(get_field name "$yaml_file")"
  if [[ -n "$name_value" ]]; then
    expected_label="sig/$name_value"
    labels_entries="$(get_list_entries labels "$yaml_file")"
    if ! printf '%s\n' "$labels_entries" | grep -qxF "$expected_label"; then
      err "labels must include $expected_label"
      fail_count=$((fail_count + 1))
    fi
    # 检查 name 是 kebab-case
    if ! [[ "$name_value" =~ $KEBAB_CASE_RE ]]; then
      err "name must be kebab-case: got '$name_value'"
      fail_count=$((fail_count + 1))
    fi
  fi

  # 检查 created_at 是 YYYY-MM-DD 格式
  created_at_value="$(get_field created_at "$yaml_file")"
  if [[ -n "$created_at_value" ]] && ! [[ "$created_at_value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
    err "created_at must be YYYY-MM-DD format: got '$created_at_value'"
    fail_count=$((fail_count + 1))
  fi

  # 检查 leads 与 maintainers 至少各有一个条目，且条目带 @
  for list_field in leads maintainers; do
    section="$(get_list_entries "$list_field" "$yaml_file")"
    if [[ -z "$section" ]]; then
      err "$list_field must have at least one entry"
      fail_count=$((fail_count + 1))
    fi
    # 每个列表条目必须以 @ 开头
    if echo "$section" | grep -vqE '^@'; then
      err "$list_field entries must be GitCode usernames starting with @"
      fail_count=$((fail_count + 1))
    fi
  done

  # 豁免注释双向一致性校验（sig-governance.md §4.2）：
  # leads 与 maintainers 为同一单人列表（各恰 1 条且相同）<=> 顶部含 "# note: 单人维护豁免期内" 前缀注释
  # 注意：§4.2 触发条件是"仓库人类 maintainer 总人数 < 2"（单人维护），完全重叠但 ≥ 2 人不构成豁免，
  # 脚本按"同一单人列表"判定，与规范语义对齐
  leads_list="$(get_list_entries leads "$yaml_file")"
  maintainers_list="$(get_list_entries maintainers "$yaml_file")"

  if [[ -n "$leads_list" && -n "$maintainers_list" ]]; then
    leads_count=$(printf '%s\n' "$leads_list" | grep -c . || true)
    maintainers_count=$(printf '%s\n' "$maintainers_list" | grep -c . || true)
    single_overlap=0
    if [[ "$leads_count" -eq 1 && "$maintainers_count" -eq 1 && "$leads_list" == "$maintainers_list" ]]; then
      single_overlap=1
    fi
    has_exemption_note=0
    if grep -qE '^#[[:space:]]*note:[[:space:]]*单人维护豁免期内' "$yaml_file"; then
      has_exemption_note=1
    fi

    if [[ "$single_overlap" -eq 1 && "$has_exemption_note" -eq 0 ]]; then
      err "leads and maintainers are the same single person but missing exemption note (must start with '# note: 单人维护豁免期内', see sig-governance.md §4.2)"
      fail_count=$((fail_count + 1))
    fi
    if [[ "$single_overlap" -eq 0 && "$has_exemption_note" -eq 1 ]]; then
      err "exemption note present but leads and maintainers are not the same single person (stale note? see sig-governance.md §4.2)"
      fail_count=$((fail_count + 1))
    fi
  fi

  # 检查 @TBD 占位符（扫描全文件，含 charter 等）
  if grep -qE '@TBD' "$yaml_file"; then
    echo "  WARN: [$yaml_file] contains @TBD placeholder (check leads/maintainers/charter)" >&2
  fi

  # 检查可选字段 scope_labels：存在时条目须为 scope/<kebab> 且非空
  if grep -qE "^scope_labels:" "$yaml_file"; then
    scope_labels_entries="$(get_list_entries scope_labels "$yaml_file")"
    if [[ -z "$scope_labels_entries" ]]; then
      err "scope_labels field present but contains no entry"
      fail_count=$((fail_count + 1))
    fi
    while IFS= read -r entry; do
      [[ -z "$entry" ]] && continue
      if ! [[ "$entry" =~ ^scope/[a-z][a-z0-9]*(-[a-z0-9]+)*$ ]]; then
        err "scope_labels entry must be 'scope/<kebab-case>': got '$entry'"
        fail_count=$((fail_count + 1))
      fi
    done <<< "$scope_labels_entries"
    all_scope_labels+="$scope_labels_entries"$'\n'
  fi

  # 检查可选字段 agents（如果存在）：逐条校验每个 entry 的子结构
  if grep -qE "^agents:" "$yaml_file"; then
    # 提取 agents 区块（从 agents: 到下一个顶级字段）
    agents_section=$(awk '
      /^agents:/ { in_agents=1; next }
      /^[a-zA-Z_]+:/ && in_agents { in_agents=0 }
      in_agents { print }
    ' "$yaml_file")

    # 按 "- id:" 切分为单个 agent entry，逐条校验 id / operator / since 均存在
    entry_count=0
    current_entry=""
    while IFS= read -r line; do
      if [[ "$line" =~ ^[[:space:]]+-[[:space:]]+id: ]]; then
        # 遇到新 entry，先校验上一个
        if [[ -n "$current_entry" ]]; then
          entry_count=$((entry_count + 1))
          for sub_field in operator since; do
            if ! echo "$current_entry" | grep -qE "^[[:space:]]+${sub_field}:"; then
              err "agents entry #$entry_count missing sub-field: $sub_field"
              fail_count=$((fail_count + 1))
            fi
          done
        fi
        current_entry="$line"$'\n'
      elif [[ -n "$current_entry" ]]; then
        current_entry+="$line"$'\n'
      fi
    done <<< "$agents_section"
    # 校验最后一个 entry
    if [[ -n "$current_entry" ]]; then
      entry_count=$((entry_count + 1))
      for sub_field in operator since; do
        if ! echo "$current_entry" | grep -qE "^[[:space:]]+${sub_field}:"; then
          err "agents entry #$entry_count missing sub-field: $sub_field"
          fail_count=$((fail_count + 1))
        fi
      done
    fi

    if [[ "$entry_count" -eq 0 ]]; then
      err "agents field present but contains no valid entry (expected list items starting with '- id:')"
      fail_count=$((fail_count + 1))
    fi

    # 校验每个 agent 的 operator 带 @
    if echo "$agents_section" | grep -E "^[[:space:]]+operator:" | grep -vqE '@'; then
      err "agents.operator must be a GitCode username starting with @"
      fail_count=$((fail_count + 1))
    fi

    # 校验每个 agent 的 id 是 kebab-case（与 name 同一标准）
    agent_ids=$(echo "$agents_section" | grep -E "^[[:space:]]+-[[:space:]]+id:" | sed -E 's/^[[:space:]]+-[[:space:]]+id:[[:space:]]*//' | tr -d '"'"'"'')
    while IFS= read -r agent_id; do
      if [[ -n "$agent_id" ]] && ! [[ "$agent_id" =~ $KEBAB_CASE_RE ]]; then
        err "agents.id must be kebab-case: got '$agent_id'"
        fail_count=$((fail_count + 1))
      fi
    done <<< "$agent_ids"

    # 校验每个 agent 的 since 是 YYYY-MM-DD 格式
    agent_sinces=$(echo "$agents_section" | grep -E "^[[:space:]]+since:" | sed -E 's/^[[:space:]]+since:[[:space:]]*//' | tr -d '"'"'"'')
    while IFS= read -r since_value; do
      if [[ -n "$since_value" ]] && ! [[ "$since_value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
        err "agents.since must be YYYY-MM-DD format: got '$since_value'"
        fail_count=$((fail_count + 1))
      fi
    done <<< "$agent_sinces"
  fi
done

# 跨文件 scope_labels 重复映射检查（sig-governance.md §6.1：一个 scope/* 原则上映射至多 1 个 SIG；
# 确需跨 SIG 统计时允许在两个 SIG 的 YAML 同时登记并在 charter 说明——故为 WARN 而非 ERROR，人工评审兜底）
dup_scope_labels=$(printf '%s' "$all_scope_labels" | grep -v '^$' | sort | uniq -d || true)
if [[ -n "$dup_scope_labels" ]]; then
  while IFS= read -r dup; do
    [[ -z "$dup" ]] && continue
    echo "  WARN: [cross-file] scope label mapped by multiple SIGs: $dup (requires charter justification, see sig-governance.md §6.1)" >&2
  done <<< "$dup_scope_labels"
fi

if [[ $file_count -eq 0 ]]; then
  echo "[validate-sig-yamls] ERROR: no yaml files found in $SIGS_DIR" >&2
  exit 1
fi

if [[ $fail_count -gt 0 ]]; then
  echo "[validate-sig-yamls] FAILED with $fail_count error(s) across $file_count file(s)" >&2
  exit 1
fi

echo "[validate-sig-yamls] OK: $file_count file(s) passed"
