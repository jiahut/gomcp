#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SKILL_SRC="${REPO_ROOT}/skills/gomcp-web-fetcher"

if [[ ! -d "${SKILL_SRC}" ]]; then
  echo "Error: skill source not found: ${SKILL_SRC}" >&2
  exit 1
fi

CODEX_HOME="${CODEX_HOME:-${HOME}/.codex}"
USER_CODEX_DIR="${CODEX_HOME}/skills"
USER_CLAUDE_DIR="${HOME}/.claude/skills"

ensure_link() {
  local target_dir="$1"
  local link_path="${target_dir}/gomcp-web-fetcher"

  mkdir -p "${target_dir}"

  if [[ -e "${link_path}" || -L "${link_path}" ]]; then
    if [[ -L "${link_path}" ]]; then
      local current
      current="$(readlink "${link_path}")"
      if [[ "${current}" == "${SKILL_SRC}" ]]; then
        echo "OK: ${link_path} already points to ${SKILL_SRC}"
        return 0
      fi
      echo "Error: ${link_path} is a symlink to ${current}, expected ${SKILL_SRC}" >&2
      exit 1
    fi
    echo "Error: ${link_path} exists and is not a symlink" >&2
    exit 1
  fi

  ln -s "${SKILL_SRC}" "${link_path}"
  echo "Linked: ${link_path} -> ${SKILL_SRC}"
}

ensure_link "${USER_CODEX_DIR}"
ensure_link "${USER_CLAUDE_DIR}"
