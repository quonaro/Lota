#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AUR_DIR="${AUR_DIR:-${ROOT_DIR}/../lota-aur}"
AUR_REPO_URL="${AUR_REPO_URL:-ssh://aur@aur.archlinux.org/lota.git}"
VERSION="${1:-${VERSION:-}}"

if [ -z "${VERSION}" ]; then
  VERSION="$(git -C "${ROOT_DIR}" describe --tags --abbrev=0 2>/dev/null || true)"
fi

if [ -z "${VERSION}" ]; then
  echo "Version is not set. Pass it as arg, VERSION env, or create a tag first."
  exit 1
fi

PKGVER="${VERSION#v}"

if [ ! -d "${AUR_DIR}/.git" ]; then
  git clone "${AUR_REPO_URL}" "${AUR_DIR}"
fi

cd "${AUR_DIR}"

sed -E -i "s/^pkgver=.*/pkgver=${PKGVER}/" PKGBUILD
sed -E -i "s/^pkgrel=.*/pkgrel=1/" PKGBUILD

rm -rf lota/ src/ pkg/

if [ "$(id -u)" = "0" ]; then
  useradd -M -s /bin/sh _build 2>/dev/null || true
  su _build -s /bin/sh -c "cd $(pwd) && makepkg --printsrcinfo" > .SRCINFO
else
  makepkg --printsrcinfo > .SRCINFO
fi

git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

git add PKGBUILD .SRCINFO

if git diff --cached --quiet; then
  echo "AUR is already up-to-date (${PKGVER})"
  exit 0
fi

git commit -m "upgpkg: lota ${PKGVER}-1"
git push origin master

echo "AUR updated to version ${PKGVER}"
