#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-${VERSION:-}}"

if [ -z "${VERSION}" ]; then
  VERSION="$(git -C "${ROOT_DIR}" describe --tags --abbrev=0 2>/dev/null || true)"
fi

if [ -z "${VERSION}" ]; then
  echo "Version is not set. Pass it as arg, VERSION env, or create a tag first."
  exit 1
fi

PKGVER="${VERSION#v}"
GITHUB_REPO="quonaro/Lota"

generate_srcinfo() {
  if [ "$(id -u)" = "0" ]; then
    useradd -m -s /bin/bash _build 2>/dev/null || true
    local tmpdir
    tmpdir=$(mktemp -d)
    cp PKGBUILD "${tmpdir}/"
    chown -R _build:_build "${tmpdir}"
    runuser -u _build -- bash -c "cd '${tmpdir}' && makepkg --printsrcinfo" > .SRCINFO
    rm -rf "${tmpdir}"
  else
    makepkg --printsrcinfo > .SRCINFO
  fi

  if [ ! -s .SRCINFO ]; then
    echo "ERROR: .SRCINFO is empty, makepkg --printsrcinfo failed" >&2
    exit 1
  fi
}

commit_and_push() {
  local pkgname="$1"
  git config user.name "github-actions[bot]"
  git config user.email "github-actions[bot]@users.noreply.github.com"
  git add PKGBUILD .SRCINFO

  if git diff --cached --quiet; then
    echo "${pkgname} AUR is already up-to-date (${PKGVER})"
    return 0
  fi

  git commit -m "upgpkg: ${pkgname} ${PKGVER}-1"
  git push origin master
  echo "${pkgname} AUR updated to version ${PKGVER}"
}

git_clone_retry() {
  local url="$1"
  local dir="$2"
  local max_retries="${GIT_CLONE_RETRIES:-10}"
  local delay="${GIT_CLONE_DELAY:-60}"
  local i

  for i in $(seq 1 "${max_retries}"); do
    if git clone "${url}" "${dir}" 2>&1; then
      return 0
    fi
    if [ "${i}" -eq "${max_retries}" ]; then
      echo "ERROR: git clone failed after ${max_retries} attempts: ${url}" >&2
      return 1
    fi
    echo "git clone attempt ${i}/${max_retries} failed, retrying in ${delay}s..." >&2
    sleep "${delay}"
  done
}

update_lota_aur() {
  local AUR_DIR="${AUR_DIR:-${ROOT_DIR}/../lota-aur}"
  local AUR_REPO_URL="${AUR_REPO_URL:-ssh://aur@aur.archlinux.org/lota.git}"

  if [ ! -d "${AUR_DIR}/.git" ]; then
    git_clone_retry "${AUR_REPO_URL}" "${AUR_DIR}"
  fi

  cd "${AUR_DIR}"
  sed -E -i "s/^pkgver=.*/pkgver=${PKGVER}/" PKGBUILD
  sed -E -i "s/^pkgrel=.*/pkgrel=1/" PKGBUILD
  rm -rf lota/ src/ pkg/

  generate_srcinfo
  commit_and_push "lota"
}

update_lota_bin() {
  local AUR_BIN_DIR="${AUR_BIN_DIR:-${ROOT_DIR}/../lota-bin}"
  local AUR_BIN_REPO_URL="${AUR_BIN_REPO_URL:-ssh://aur@aur.archlinux.org/lota-bin.git}"

  if [ ! -d "${AUR_BIN_DIR}/.git" ]; then
    git_clone_retry "${AUR_BIN_REPO_URL}" "${AUR_BIN_DIR}"
  fi

  cd "${AUR_BIN_DIR}"

  local checksums_url="https://github.com/${GITHUB_REPO}/releases/download/v${PKGVER}/checksums.txt"
  local checksums
  checksums=$(curl -fsSL "${checksums_url}")
  if [ -z "${checksums}" ]; then
    echo "ERROR: Failed to download checksums from ${checksums_url}" >&2
    exit 1
  fi

  local sha256_amd64 sha256_arm64
  sha256_amd64=$(echo "${checksums}" | awk '/lota-linux-amd64$/ {print $1}')
  sha256_arm64=$(echo "${checksums}" | awk '/lota-linux-arm64$/ {print $1}')

  if [ -z "${sha256_amd64}" ] || [ -z "${sha256_arm64}" ]; then
    echo "ERROR: Could not find sha256sums in checksums.txt" >&2
    echo "${checksums}" >&2
    exit 1
  fi

  sed -E -i "s/^pkgver=.*/pkgver=${PKGVER}/" PKGBUILD
  sed -E -i "s/^pkgrel=.*/pkgrel=1/" PKGBUILD
  sed -E -i "s/^sha256sums_x86_64=.*/sha256sums_x86_64=('${sha256_amd64}')/" PKGBUILD
  sed -E -i "s/^sha256sums_aarch64=.*/sha256sums_aarch64=('${sha256_arm64}')/" PKGBUILD

  rm -rf src/ pkg/ lota-bin-*/

  generate_srcinfo
  commit_and_push "lota-bin"
}

update_lota_aur
update_lota_bin
