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
#     - 必需字段存在性、status 枚举、labels 含 sig/<name>
#     - name 的 kebab-case 格式
#     - created_at / agents[].since 的 YYYY-MM-DD 日期格式
#     - leads / maintainers 非空且条目带 @
#     - 豁免注释双向一致性（leads 与 maintainers 完全重叠 <=> 顶部含 "# note: 单人维护豁免期内" 前缀注释）
#     - agents 每个条目均含 id / operator / since，id 为 kebab-case，operator 带 @
#
# 已知缺口（脚本不校验，由 PR review 人工兜底，见 sig-governance.md §5.3）：
#     - agents[].scopes 是否为 scope 字段的子集
#     - agents[].operator 是否在本 YAML 的 leads/maintainers 列表中（更严格的"必须是仓库 maintainer"无注册表可查）
#     - scope 路径是否真实存在于仓库
#     - leads/maintainers 条目非空值（仅检查列表非空与 @ 前缀）

set -euo pipefail

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

fail_count=0
file_count=0

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
      echo "  ERROR: missing required field: $field" >&2
      fail_count=$((fail_count + 1))
    fi
  done

  # 检查 status 值
  status_value=$(grep -E "^status:" "$yaml_file" | head -n1 | sed -E 's/^status:[[:space:]]*//' | tr -d '"'"'"'')
  if [[ -n "$status_value" ]] && ! [[ "$status_value" =~ ^($ALLOWED_STATUS)$ ]]; then
    echo "  ERROR: invalid status value: $status_value (allowed: $ALLOWED_STATUS)" >&2
    fail_count=$((fail_count + 1))
  fi

  # 检查 labels 中是否包含 sig/<name>
  name_value=$(grep -E "^name:" "$yaml_file" | head -n1 | sed -E 's/^name:[[:space:]]*//' | tr -d '"'"'"'')
  if [[ -n "$name_value" ]]; then
    expected_label="sig/$name_value"
    if ! grep -qE "^[[:space:]]*-[[:space:]]*${expected_label}\b" "$yaml_file"; then
      echo "  ERROR: labels must include $expected_label" >&2
      fail_count=$((fail_count + 1))
    fi
    # 检查 name 是 kebab-case
    if ! [[ "$name_value" =~ ^[a-z0-9]+(-[a-z0-9]+)*$ ]]; then
      echo "  ERROR: name must be kebab-case: got '$name_value'" >&2
      fail_count=$((fail_count + 1))
    fi
  fi

  # 检查 created_at 是 YYYY-MM-DD 格式
  created_at_value=$(grep -E "^created_at:" "$yaml_file" | head -n1 | sed -E 's/^created_at:[[:space:]]*//' | tr -d '"'"'"'')
  if [[ -n "$created_at_value" ]] && ! [[ "$created_at_value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
    echo "  ERROR: created_at must be YYYY-MM-DD format: got '$created_at_value'" >&2
    fail_count=$((fail_count + 1))
  fi

  # 检查 leads 与 maintainers 至少各有一个条目，且条目带 @
  for list_field in leads maintainers; do
    # 提取 leads: 到下一个顶级字段之间的内容
    section=$(awk -v f="^${list_field}:" '
      $0 ~ f { in_section=1; next }
      /^[a-zA-Z_]+:/ && in_section { in_section=0 }
      in_section { print }
    ' "$yaml_file")
    if ! echo "$section" | grep -qE '^[[:space:]]*-'; then
      echo "  ERROR: $list_field must have at least one entry" >&2
      fail_count=$((fail_count + 1))
    fi
    # 每个列表条目必须以 @ 开头（允许引号包裹）
    if echo "$section" | grep -E '^[[:space:]]*-' | grep -vqE '^[[:space:]]*-[[:space:]]*["'"'"']?@'; then
      echo "  ERROR: $list_field entries must be GitCode usernames starting with @" >&2
      fail_count=$((fail_count + 1))
    fi
  done

  # 豁免注释双向一致性校验（sig-governance.md §4.1/§4.2）：
  # leads 与 maintainers 完全重叠 <=> 文件顶部必须含 "# note: 单人维护豁免期内" 前缀注释
  leads_list=$(awk '
    /^leads:/ { in_section=1; next }
    /^[a-zA-Z_]+:/ && in_section { in_section=0 }
    in_section { print }
  ' "$yaml_file" | grep -E '^[[:space:]]*-' | sed -E 's/^[[:space:]]*-[[:space:]]*//; s/["'"'"']//g' | sort)
  maintainers_list=$(awk '
    /^maintainers:/ { in_section=1; next }
    /^[a-zA-Z_]+:/ && in_section { in_section=0 }
    in_section { print }
  ' "$yaml_file" | grep -E '^[[:space:]]*-' | sed -E 's/^[[:space:]]*-[[:space:]]*//; s/["'"'"']//g' | sort)

  if [[ -n "$leads_list" && -n "$maintainers_list" ]]; then
    overlap=0
    if [[ "$leads_list" == "$maintainers_list" ]]; then
      overlap=1
    fi
    has_exemption_note=0
    if grep -qE '^#[[:space:]]*note:[[:space:]]*单人维护豁免期内' "$yaml_file"; then
      has_exemption_note=1
    fi

    if [[ "$overlap" -eq 1 && "$has_exemption_note" -eq 0 ]]; then
      echo "  ERROR: leads and maintainers fully overlap but missing exemption note (must start with '# note: 单人维护豁免期内', see sig-governance.md §4.2)" >&2
      fail_count=$((fail_count + 1))
    fi
    if [[ "$overlap" -eq 0 && "$has_exemption_note" -eq 1 ]]; then
      echo "  ERROR: exemption note present but leads and maintainers do not fully overlap (stale note? see sig-governance.md §4.2)" >&2
      fail_count=$((fail_count + 1))
    fi
  fi

  # 检查 leads 不包含 @TBD
  if grep -qE '@TBD' "$yaml_file"; then
    echo "  WARN: leads/maintainers contain @TBD placeholder" >&2
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
              echo "  ERROR: agents entry #$entry_count missing sub-field: $sub_field" >&2
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
          echo "  ERROR: agents entry #$entry_count missing sub-field: $sub_field" >&2
          fail_count=$((fail_count + 1))
        fi
      done
    fi

    if [[ "$entry_count" -eq 0 ]]; then
      echo "  ERROR: agents field present but contains no valid entry (expected list items starting with '- id:')" >&2
      fail_count=$((fail_count + 1))
    fi

    # 校验每个 agent 的 operator 带 @
    if echo "$agents_section" | grep -E "^[[:space:]]+operator:" | grep -vqE '@'; then
      echo "  ERROR: agents.operator must be a GitCode username starting with @" >&2
      fail_count=$((fail_count + 1))
    fi

    # 校验每个 agent 的 id 是 kebab-case
    agent_ids=$(echo "$agents_section" | grep -E "^[[:space:]]+-[[:space:]]+id:" | sed -E 's/^[[:space:]]+-[[:space:]]+id:[[:space:]]*//' | tr -d '"'"'"'')
    while IFS= read -r agent_id; do
      if [[ -n "$agent_id" ]] && ! [[ "$agent_id" =~ ^[a-z][a-z0-9-]*$ ]]; then
        echo "  ERROR: agents.id must be kebab-case: got '$agent_id'" >&2
        fail_count=$((fail_count + 1))
      fi
    done <<< "$agent_ids"

    # 校验每个 agent 的 since 是 YYYY-MM-DD 格式
    agent_sinces=$(echo "$agents_section" | grep -E "^[[:space:]]+since:" | sed -E 's/^[[:space:]]+since:[[:space:]]*//' | tr -d '"'"'"'')
    while IFS= read -r since_value; do
      if [[ -n "$since_value" ]] && ! [[ "$since_value" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
        echo "  ERROR: agents.since must be YYYY-MM-DD format: got '$since_value'" >&2
        fail_count=$((fail_count + 1))
      fi
    done <<< "$agent_sinces"
  fi
done

if [[ $file_count -eq 0 ]]; then
  echo "[validate-sig-yamls] ERROR: no yaml files found in $SIGS_DIR" >&2
  exit 1
fi

if [[ $fail_count -gt 0 ]]; then
  echo "[validate-sig-yamls] FAILED with $fail_count error(s) across $file_count file(s)" >&2
  exit 1
fi

echo "[validate-sig-yamls] OK: $file_count file(s) passed"
