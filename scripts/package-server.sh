#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist/packages"
WORK_DIR="${ROOT_DIR}/tmp/package-server"
VERSION="${VERSION:-$(cat "$ROOT_DIR/VERSION")}"
RELEASE="${RELEASE:-1}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
PACKAGE_FORMAT="${1:-all}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"

case "$PACKAGE_FORMAT" in
  all|deb|rpm) ;;
  *) echo "usage: $0 [all|deb|rpm]" >&2; exit 2 ;;
esac

deb_arch() {
  case "$GOARCH" in
    amd64) echo "amd64" ;;
    arm64) echo "arm64" ;;
    arm) echo "armhf" ;;
    *) echo "$GOARCH" ;;
  esac
}

rpm_arch() {
  case "$GOARCH" in
    amd64) echo "x86_64" ;;
    arm64) echo "aarch64" ;;
    arm) echo "armv7hl" ;;
    *) echo "$GOARCH" ;;
  esac
}

rpm_version() {
  echo "$VERSION" | sed 's/^[vV]//' | tr '-' '_'
}

install_payload() {
  local root="$1"
  local unit_dir="$2"

  install -d "$root/usr/bin"
  install -d "$root/usr/lib/s3s5"
  install -d "$root/etc/s3s5"
  install -d "$root/$unit_dir"
  install -d "$root/usr/share/doc/s3s5-server"

  install -m 0755 "$WORK_DIR/s3s5-server" "$root/usr/bin/s3s5-server"
  install -m 0755 "$ROOT_DIR/packaging/systemd/s3s5-server-start" "$root/usr/lib/s3s5/s3s5-server-start"
  install -m 0644 "$ROOT_DIR/packaging/systemd/s3s5-server.service" "$root/$unit_dir/s3s5-server.service"
  install -m 0600 "$ROOT_DIR/packaging/systemd/s3s5-server.env" "$root/etc/s3s5/s3s5-server.env"
  install -m 0644 "$ROOT_DIR/README.md" "$root/usr/share/doc/s3s5-server/README.md"
}

build_binary() {
  mkdir -p "$WORK_DIR"
  (
    cd "$ROOT_DIR"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
      -trimpath \
      -ldflags="-s -w -X s3s5/internal/version.Version=${VERSION} -X s3s5/internal/version.Commit=${COMMIT} -X s3s5/internal/version.Date=${DATE}" \
      -o "$WORK_DIR/s3s5-server" \
      ./cmd/s3s5-server
  )
}

build_deb() {
  local arch package root
  arch="$(deb_arch)"
  package="s3s5-server_${VERSION}-${RELEASE}_${arch}.deb"
  root="$WORK_DIR/deb-root"
  rm -rf "$root"
  install_payload "$root" "lib/systemd/system"
  install -d "$root/DEBIAN"
  cat > "$root/DEBIAN/control" <<EOF
Package: s3s5-server
Version: ${VERSION}-${RELEASE}
Section: net
Priority: optional
Architecture: ${arch}
Maintainer: s3s5 maintainers
Depends: bash, adduser, systemd
Description: SOCKS5-over-S3 server
 s3s5-server polls an S3-compatible object store and connects approved TCP
 targets for SOCKS5-over-S3 client sessions.
EOF
  cat > "$root/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if ! getent group s3s5 >/dev/null; then
  addgroup --system s3s5 >/dev/null 2>&1 || groupadd --system s3s5
fi
if ! getent passwd s3s5 >/dev/null; then
  adduser --system --ingroup s3s5 --no-create-home --home /nonexistent --shell /usr/sbin/nologin s3s5 >/dev/null 2>&1 \
    || useradd --system --gid s3s5 --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin s3s5
fi
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
exit 0
EOF
  cat > "$root/DEBIAN/postrm" <<'EOF'
#!/bin/sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi
exit 0
EOF
  cat > "$root/DEBIAN/conffiles" <<'EOF'
/etc/s3s5/s3s5-server.env
EOF
  chmod 0755 "$root/DEBIAN/postinst" "$root/DEBIAN/postrm"
  mkdir -p "$DIST_DIR"
  dpkg-deb --root-owner-group --build "$root" "$DIST_DIR/$package"
}

build_rpm() {
  local arch top spec package root rv
  arch="$(rpm_arch)"
  rv="$(rpm_version)"
  top="$WORK_DIR/rpmbuild"
  root="$WORK_DIR/rpm-root"
  spec="$top/SPECS/s3s5-server.spec"
  rm -rf "$top" "$root"
  mkdir -p "$top/BUILD" "$top/BUILDROOT" "$top/RPMS" "$top/SOURCES" "$top/SPECS" "$top/SRPMS"
  install_payload "$root" "usr/lib/systemd/system"
  cat > "$spec" <<EOF
Name: s3s5-server
Version: ${rv}
Release: ${RELEASE}%{?dist}
Summary: SOCKS5-over-S3 server
License: Apache-2.0
BuildArch: ${arch}
Requires: bash
Requires(pre): shadow-utils
Requires(post): systemd
Requires(postun): systemd

%description
s3s5-server polls an S3-compatible object store and connects approved TCP
targets for SOCKS5-over-S3 client sessions.

%prep

%build

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}
cp -a ${root}/. %{buildroot}/

%pre
getent group s3s5 >/dev/null || groupadd -r s3s5
getent passwd s3s5 >/dev/null || useradd -r -g s3s5 -d /nonexistent -s /sbin/nologin -M s3s5

%post
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

%postun
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

%files
%attr(0755,root,root) /usr/bin/s3s5-server
%attr(0755,root,root) /usr/lib/s3s5/s3s5-server-start
%attr(0644,root,root) /usr/lib/systemd/system/s3s5-server.service
%config(noreplace) %attr(0600,root,root) /etc/s3s5/s3s5-server.env
%doc /usr/share/doc/s3s5-server/README.md
EOF
  rpmbuild --define "_topdir $top" -bb "$spec"
  mkdir -p "$DIST_DIR"
  package="$(find "$top/RPMS" -type f -name '*.rpm' | head -n 1)"
  cp "$package" "$DIST_DIR/"
}

rm -rf "$WORK_DIR"
mkdir -p "$DIST_DIR"
rm -f "$DIST_DIR"/s3s5-server_*.deb "$DIST_DIR"/s3s5-server-*.rpm
build_binary

case "$PACKAGE_FORMAT" in
  all)
    build_deb
    build_rpm
    ;;
  deb) build_deb ;;
  rpm) build_rpm ;;
esac

find "$DIST_DIR" -maxdepth 1 -type f \( -name '*.deb' -o -name '*.rpm' \) -print | sort
