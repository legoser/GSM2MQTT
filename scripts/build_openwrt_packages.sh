#!/usr/bin/env bash
# ==============================================================================
# build_openwrt_packages.sh — Build OPKG (.ipk) and APK (.apk) packages
# for OpenWrt deployment.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PKG_NAME="gsm2mqtt"
PKG_DESC="GSM to MQTT gateway (SMS, USSD, modem management)"
PKG_URL="https://github.com/legoser/gsm2mqtt"
PKG_LICENSE="Apache-2.0"
PKG_MAINTAINER="GSM2MQTT Authors <https://github.com/legoser/gsm2mqtt>"

VERSION=""
TARGET_TYPE="all"     # ipk | apk | all
TARGET_ARCH="all"     # amd64 | arm64 | riscv64 | armv7 | mipsel | mips | all
BIN_DIR="${REPO_ROOT}/bin"
OUT_DIR="${REPO_ROOT}/dist"

usage() {
    cat << EOF
Usage: $(basename "$0") [options]

Build OpenWrt packages in OPKG (.ipk) and/or APK (.apk) format.

Options:
  --type TYPE       Package format to build: ipk, apk, all (default: all)
  --arch ARCH       Architecture: amd64, arm64, riscv64, armv7, mipsel, mips, all (default: all)
  --version VER     Package version (default: git tag/describe or '0.1.0')
  --bin-dir DIR     Directory containing or receiving compiled binaries (default: bin)
  --out-dir DIR     Output directory for packages (default: dist)
  -h, --help        Show this help message
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --type)
            TARGET_TYPE="$2"
            shift 2
            ;;
        --arch)
            TARGET_ARCH="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --bin-dir)
            BIN_DIR="$2"
            shift 2
            ;;
        --out-dir)
            OUT_DIR="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage
            ;;
    esac
done

# Resolve version
if [[ -z "$VERSION" ]]; then
    VERSION="$(git describe --tags --always 2>/dev/null || echo "0.1.0")"
fi

# Clean version for OPKG and APK
# Raw tag like v1.2.3 -> 1.2.3
RAW_VER="${VERSION#v}"
RAW_VER="${RAW_VER%%-*}" # base semver if git describe suffix exists
if [[ ! "$RAW_VER" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
    RAW_VER="0.1.0"
fi

OPKG_VER="${RAW_VER}-1"
APK_VER="${RAW_VER}-r1"

# Check tar flags support
TAR_OWNER_FLAGS=()
if tar --help 2>&1 | grep -q -- '--owner'; then
    TAR_OWNER_FLAGS=(--owner=0 --group=0 --numeric-owner)
fi

mkdir -p "$BIN_DIR" "$OUT_DIR"
BIN_DIR="$(cd "$BIN_DIR" && pwd)"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

compile_binary() {
    local arch="$1"
    local bin_path="$BIN_DIR/${PKG_NAME}-linux-${arch}"

    if [[ -f "$bin_path" ]]; then
        echo "=> Binary $bin_path already exists, skipping compilation."
        return 0
    fi

    echo "=> Compiling ${PKG_NAME} for linux/${arch}..."
    local build_time
    build_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    local ldflags="-s -w -X github.com/legoser/gsm2mqtt/internal/version.Version=${VERSION} -X github.com/legoser/gsm2mqtt/internal/version.BuildTime=${build_time} -X main.version=${VERSION} -X main.buildTime=${build_time}"

    case "$arch" in
        amd64)
            CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$bin_path" "${REPO_ROOT}/cmd/gsm2mqtt/"
            ;;
        arm64)
            CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$ldflags" -o "$bin_path" "${REPO_ROOT}/cmd/gsm2mqtt/"
            ;;
        riscv64)
            CGO_ENABLED=0 GOOS=linux GOARCH=riscv64 go build -trimpath -ldflags "$ldflags" -o "$bin_path" "${REPO_ROOT}/cmd/gsm2mqtt/"
            ;;
        armv7)
            CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags "$ldflags" -o "$bin_path" "${REPO_ROOT}/cmd/gsm2mqtt/"
            ;;
        mipsel)
            CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -trimpath -ldflags "$ldflags" -o "$bin_path" "${REPO_ROOT}/cmd/gsm2mqtt/"
            ;;
        mips)
            CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat go build -trimpath -ldflags "$ldflags" -o "$bin_path" "${REPO_ROOT}/cmd/gsm2mqtt/"
            ;;
        *)
            echo "Unknown architecture: $arch" >&2
            return 1
            ;;
    esac
}

build_ipk() {
    local arch="$1"
    local opkg_arch="$2"
    local bin_path="$BIN_DIR/${PKG_NAME}-linux-${arch}"

    echo "==> Building OPKG (.ipk) for ${arch} (${opkg_arch})..."
    local stage
    stage="$(mktemp -d)"

    local data_dir="${stage}/data"
    local ctrl_dir="${stage}/control"

    mkdir -p "${data_dir}/usr/bin" \
             "${data_dir}/etc/init.d" \
             "${data_dir}/etc/config" \
             "${data_dir}/etc/gsm2mqtt" \
             "${ctrl_dir}"

    cp "$bin_path" "${data_dir}/usr/bin/${PKG_NAME}"
    chmod 755 "${data_dir}/usr/bin/${PKG_NAME}"

    cp "${REPO_ROOT}/deployments/openwrt/files/gsm2mqtt.init" "${data_dir}/etc/init.d/${PKG_NAME}"
    chmod 755 "${data_dir}/etc/init.d/${PKG_NAME}"

    cp "${REPO_ROOT}/deployments/openwrt/files/gsm2mqtt.config" "${data_dir}/etc/config/${PKG_NAME}"
    chmod 644 "${data_dir}/etc/config/${PKG_NAME}"

    cp "${REPO_ROOT}/deployments/openwrt/files/gsm2mqtt.yaml" "${data_dir}/etc/gsm2mqtt/${PKG_NAME}.yaml"
    chmod 640 "${data_dir}/etc/gsm2mqtt/${PKG_NAME}.yaml"

    # Control metadata
    cat << EOF > "${ctrl_dir}/control"
Package: ${PKG_NAME}
Version: ${OPKG_VER}
Depends:
Section: net
Architecture: ${opkg_arch}
Maintainer: ${PKG_MAINTAINER}
Description: ${PKG_DESC}
EOF

    cat << EOF > "${ctrl_dir}/conffiles"
/etc/config/${PKG_NAME}
/etc/gsm2mqtt/${PKG_NAME}.yaml
EOF

    cat << 'EOF' > "${ctrl_dir}/postinst"
#!/bin/sh
mkdir -p /var/lock /var/run
[ -z "$IPKG_INSTROOT" ] && [ -x /etc/init.d/gsm2mqtt ] && {
    /etc/init.d/gsm2mqtt enable
}
exit 0
EOF
    chmod 755 "${ctrl_dir}/postinst"

    cat << 'EOF' > "${ctrl_dir}/prerm"
#!/bin/sh
mkdir -p /var/lock /var/run
[ -z "$IPKG_INSTROOT" ] && [ -x /etc/init.d/gsm2mqtt ] && {
    /etc/init.d/gsm2mqtt disable
    /etc/init.d/gsm2mqtt stop 2>/dev/null || true
}
exit 0
EOF
    chmod 755 "${ctrl_dir}/prerm"

    echo "2.0" > "${stage}/debian-binary"

    tar -czf "${stage}/data.tar.gz" "${TAR_OWNER_FLAGS[@]}" -C "${data_dir}" .
    tar -czf "${stage}/control.tar.gz" "${TAR_OWNER_FLAGS[@]}" -C "${ctrl_dir}" .

    local out_file="${OUT_DIR}/${PKG_NAME}_${OPKG_VER}_${opkg_arch}.ipk"
    tar -czf "$out_file" "${TAR_OWNER_FLAGS[@]}" -C "${stage}" debian-binary control.tar.gz data.tar.gz

    rm -rf "$stage"
    echo "    Created: ${out_file}"
}

build_apk() {
    local arch="$1"
    local apk_arch="$2"
    local bin_path="$BIN_DIR/${PKG_NAME}-linux-${arch}"

    echo "==> Building APK (.apk) for ${arch} (${apk_arch})..."
    local stage
    stage="$(mktemp -d)"

    local root_dir="${stage}/root"
    local scripts_dir="${stage}/scripts"

    mkdir -p "${root_dir}/usr/bin" \
             "${root_dir}/etc/init.d" \
             "${root_dir}/etc/config" \
             "${root_dir}/etc/gsm2mqtt" \
             "${scripts_dir}"

    cp "$bin_path" "${root_dir}/usr/bin/${PKG_NAME}"
    chmod 755 "${root_dir}/usr/bin/${PKG_NAME}"

    cp "${REPO_ROOT}/deployments/openwrt/files/gsm2mqtt.init" "${root_dir}/etc/init.d/${PKG_NAME}"
    chmod 755 "${root_dir}/etc/init.d/${PKG_NAME}"

    cp "${REPO_ROOT}/deployments/openwrt/files/gsm2mqtt.config" "${root_dir}/etc/config/${PKG_NAME}"
    chmod 644 "${root_dir}/etc/config/${PKG_NAME}"

    cp "${REPO_ROOT}/deployments/openwrt/files/gsm2mqtt.yaml" "${root_dir}/etc/gsm2mqtt/${PKG_NAME}.yaml"
    chmod 640 "${root_dir}/etc/gsm2mqtt/${PKG_NAME}.yaml"

    cat << 'EOF' > "${scripts_dir}/post-install.sh"
#!/bin/sh
mkdir -p /var/lock /var/run
[ -x /etc/init.d/gsm2mqtt ] && /etc/init.d/gsm2mqtt enable
exit 0
EOF
    chmod 755 "${scripts_dir}/post-install.sh"

    cat << 'EOF' > "${scripts_dir}/pre-deinstall.sh"
#!/bin/sh
mkdir -p /var/lock /var/run
[ -x /etc/init.d/gsm2mqtt ] && {
    /etc/init.d/gsm2mqtt disable
    /etc/init.d/gsm2mqtt stop 2>/dev/null || true
}
exit 0
EOF
    chmod 755 "${scripts_dir}/pre-deinstall.sh"

    local out_file="${OUT_DIR}/${PKG_NAME}-${APK_VER}-${apk_arch}.apk"

    if command -v apk >/dev/null 2>&1 && apk mkpkg --help >/dev/null 2>&1; then
        apk mkpkg \
            --files "${root_dir}" \
            --info "name:${PKG_NAME}" \
            --info "version:${APK_VER}" \
            --info "arch:${apk_arch}" \
            --info "description:${PKG_DESC}" \
            --info "license:${PKG_LICENSE}" \
            --info "url:${PKG_URL}" \
            --script "post-install:${scripts_dir}/post-install.sh" \
            --script "pre-deinstall:${scripts_dir}/pre-deinstall.sh" \
            -o "$out_file"
    elif command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
        docker run --rm \
            -v "${stage}:/stage:rw" \
            -v "${OUT_DIR}:/out:rw" \
            alpine:latest sh -c "
                apk mkpkg \
                    --files /stage/root \
                    --info 'name:${PKG_NAME}' \
                    --info 'version:${APK_VER}' \
                    --info 'arch:${apk_arch}' \
                    --info 'description:${PKG_DESC}' \
                    --info 'license:${PKG_LICENSE}' \
                    --info 'url:${PKG_URL}' \
                    --script 'post-install:/stage/scripts/post-install.sh' \
                    --script 'pre-deinstall:/stage/scripts/pre-deinstall.sh' \
                    -o '/out/$(basename "$out_file")'
            "
    else
        echo "Error: apk-tools (with 'apk mkpkg') or Docker is required to build APK packages." >&2
        rm -rf "$stage"
        return 1
    fi

    rm -rf "$stage"
    echo "    Created: ${out_file}"
}

# Selected arches
ARCH_LIST=()
if [[ "$TARGET_ARCH" == "all" ]]; then
    ARCH_LIST=(amd64 arm64 riscv64 armv7 mipsel mips)
else
    ARCH_LIST=("$TARGET_ARCH")
fi

echo "=================================================="
echo " Building OpenWrt packages for: ${PKG_NAME}"
echo " Version: ${VERSION} (OPKG: ${OPKG_VER}, APK: ${APK_VER})"
echo " Format:  ${TARGET_TYPE}"
echo " Arches:  ${ARCH_LIST[*]}"
echo " Output:  ${OUT_DIR}"
echo "=================================================="

for arch in "${ARCH_LIST[@]}"; do
    compile_binary "$arch"

    # Map architecture names
    case "$arch" in
        amd64)
            opkg_arch="x86_64"
            apk_arch="x86_64"
            ;;
        arm64)
            opkg_arch="aarch64_generic"
            apk_arch="aarch64"
            ;;
        riscv64)
            opkg_arch="riscv64"
            apk_arch="riscv64"
            ;;
        armv7)
            opkg_arch="arm_cortex-a7_neon-vfpv4"
            apk_arch="armv7"
            ;;
        mipsel)
            opkg_arch="mipsel_24kc"
            apk_arch="mipsel"
            ;;
        mips)
            opkg_arch="mips_24kc"
            apk_arch="mips"
            ;;
        *)
            echo "Unsupported architecture: $arch" >&2
            exit 1
            ;;
    esac

    if [[ "$TARGET_TYPE" == "ipk" || "$TARGET_TYPE" == "all" ]]; then
        build_ipk "$arch" "$opkg_arch"
    fi

    if [[ "$TARGET_TYPE" == "apk" || "$TARGET_TYPE" == "all" ]]; then
        build_apk "$arch" "$apk_arch"
    fi
done

echo "=================================================="
echo "OpenWrt packages generated successfully in ${OUT_DIR}:"
ls -la "${OUT_DIR}"/*.*pk 2>/dev/null || true
echo "=================================================="
