#!/bin/sh
# ==============================================================================
# GSM2MQTT Universal One-Line Installer
#
# Supported OS:
#   - OpenWrt (OPKG / APK / standalone procd)
#   - Linux with systemd (Ubuntu, Debian, Raspberry Pi OS, Armbian, Arch, Fedora)
#   - Alpine Linux (OpenRC)
#   - Generic Linux (standalone binary)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/legoser/gsm2mqtt/main/scripts/install.sh | sudo sh
#
# On OpenWrt (as root):
#   wget -qO- https://raw.githubusercontent.com/legoser/gsm2mqtt/main/scripts/install.sh | sh
#
# Optional environment overrides:
#   VERSION=v0.1.4    # Specific release version (default: latest)
#   REPO=owner/repo   # Alternative GitHub repository (default: legoser/gsm2mqtt)
#   BIN_DIR=/usr/bin  # Custom binary install location
# ==============================================================================

set -eu

REPO="${REPO:-legoser/gsm2mqtt}"
VERSION="${VERSION:-latest}"

# Color output helpers (disable if stdout is not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    NC=''
fi

info() {
    printf "${BLUE}${BOLD}==>${NC} %s\n" "$1"
}

success() {
    printf "${GREEN}${BOLD}✓${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}${BOLD}!${NC} %s\n" "$1"
}

error() {
    printf "${RED}${BOLD}Error:${NC} %s\n" "$1" >&2
}

abort() {
    error "$1"
    exit 1
}

# 1. Verify root privileges
if [ "$(id -u)" -ne 0 ]; then
    abort "This installer must be run as root. Try: sudo sh -c \"\$(curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh)\""
fi

# 2. Check download tool
FETCH_CMD=""
if command -v curl >/dev/null 2>&1; then
    FETCH_CMD="curl"
elif command -v wget >/dev/null 2>&1; then
    FETCH_CMD="wget"
else
    abort "Neither 'curl' nor 'wget' was found. Please install one to proceed."
fi

fetch() {
    local url="$1"
    local dest="$2"
    if [ "$FETCH_CMD" = "curl" ]; then
        curl -fsSL "$url" -o "$dest"
    else
        wget --no-check-certificate -q -O "$dest" "$url"
    fi
}

get_latest_tag() {
    local tag=""
    if [ "$FETCH_CMD" = "curl" ]; then
        tag="$(curl -sSLI "https://github.com/${REPO}/releases/latest" 2>/dev/null | grep -i '^location:' | sed 's/.*tag\///' | head -n 1 | tr -d '\r\n' || true)"
        if [ -z "$tag" ]; then
            tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1 || true)"
        fi
    else
        tag="$(wget --no-check-certificate -q -O - "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1 || true)"
    fi
    echo "$tag"
}

# 3. Detect operating system and init system
detect_system() {
    if [ -f /etc/openwrt_release ] || [ -f /etc/openwrt_version ] || command -v uci >/dev/null 2>&1; then
        echo "openwrt"
    elif [ -d /run/systemd/system ] || command -v systemctl >/dev/null 2>&1; then
        echo "systemd"
    elif [ -f /etc/alpine-release ] || command -v rc-service >/dev/null 2>&1; then
        echo "openrc"
    else
        echo "generic"
    fi
}

SYS_TYPE="$(detect_system)"
HOST_ARCH="$(uname -m)"

info "Detected platform: ${SYS_TYPE} (${HOST_ARCH})"

# 4. Resolve target release version
TAG="$VERSION"
if [ "$TAG" = "latest" ]; then
    info "Resolving latest release tag from GitHub (${REPO})..."
    TAG="$(get_latest_tag)"
    if [ -z "$TAG" ]; then
        abort "Failed to determine latest release tag. You can specify a version explicitly: VERSION=v0.1.4 sh install.sh"
    fi
fi

RAW_VER="${TAG#v}"
info "Installing GSM2MQTT ${TAG} (raw version: ${RAW_VER})..."

TMP_DIR="$(mktemp -d -t gsm2mqtt-install.XXXXXX 2>/dev/null || mktemp -d /tmp/gsm2mqtt-install.XXXXXX)"
trap 'rm -rf "${TMP_DIR}"' EXIT INT TERM

# ------------------------------------------------------------------------------
# Installation on OpenWrt (OPKG / APK / Standalone procd)
# ------------------------------------------------------------------------------
install_openwrt() {
    local installed=0
    mkdir -p /var/lock /var/run /var/run/gsm2mqtt

    # Option A: OPKG (OpenWrt <= 23.05)
    if command -v opkg >/dev/null 2>&1; then
        info "OpenWrt OPKG package manager detected."
        local detected_arch
        detected_arch="$(opkg print-architecture 2>/dev/null | awk 'NF>=3 {val=$3+0; if (val > max) {max=val; arch=$2}} END {print arch}')"
        if [ -z "$detected_arch" ]; then
            case "$HOST_ARCH" in
                x86_64) detected_arch="x86_64" ;;
                aarch64) detected_arch="aarch64_generic" ;;
                armv7*|armv6*) detected_arch="arm_cortex-a7_neon-vfpv4" ;;
                mips*le*) detected_arch="mipsel_24kc" ;;
                mips*) detected_arch="mips_24kc" ;;
                riscv64) detected_arch="riscv64" ;;
                *) detected_arch="x86_64" ;;
            esac
        fi

        info "Primary device OPKG architecture: ${detected_arch}"
        local pkg_name="gsm2mqtt_${RAW_VER}-1_${detected_arch}.ipk"
        local ipk_url="https://github.com/${REPO}/releases/download/${TAG}/${pkg_name}"
        local ipk_dest="${TMP_DIR}/gsm2mqtt.ipk"

        info "Downloading ${pkg_name}..."
        if fetch "$ipk_url" "$ipk_dest" 2>/dev/null; then
            info "Installing ${pkg_name} via opkg..."
            if opkg install "$ipk_dest"; then
                installed=1
            fi
        else
            # Try fallback to generic architecture if specific sub-arch was not in release assets
            if [ "$detected_arch" = "aarch64_cortex-a53" ] || [ "$detected_arch" = "aarch64_cortex-a72" ]; then
                warn "Sub-architecture package ${pkg_name} not found, trying aarch64_generic..."
                local fallback_name="gsm2mqtt_${RAW_VER}-1_aarch64_generic.ipk"
                local fallback_url="https://github.com/${REPO}/releases/download/${TAG}/${fallback_name}"
                if fetch "$fallback_url" "$ipk_dest" 2>/dev/null; then
                    info "Installing ${fallback_name} with --force-architecture..."
                    if opkg install --force-architecture "$ipk_dest"; then
                        installed=1
                    fi
                fi
            fi
        fi
    fi

    # Option B: APK (OpenWrt >= 25.12)
    if [ "$installed" -eq 0 ] && command -v apk >/dev/null 2>&1; then
        info "OpenWrt APK package manager detected."
        local apk_arch
        case "$HOST_ARCH" in
            x86_64) apk_arch="x86_64" ;;
            aarch64) apk_arch="aarch64" ;;
            armv7*|armv6*) apk_arch="armv7" ;;
            mips*le*) apk_arch="mipsel" ;;
            mips*) apk_arch="mips" ;;
            riscv64) apk_arch="riscv64" ;;
            *) apk_arch="$HOST_ARCH" ;;
        esac

        local apk_name="gsm2mqtt-${RAW_VER}-r1-${apk_arch}.apk"
        local apk_url="https://github.com/${REPO}/releases/download/${TAG}/${apk_name}"
        local apk_dest="${TMP_DIR}/gsm2mqtt.apk"

        info "Downloading ${apk_name}..."
        if fetch "$apk_url" "$apk_dest" 2>/dev/null; then
            info "Installing ${apk_name} via apk..."
            if apk add --allow-untrusted "$apk_dest"; then
                installed=1
            fi
        fi
    fi

    # Option C: Standalone binary fallback on OpenWrt
    if [ "$installed" -eq 0 ]; then
        warn "Package manager installation skipped or unavailable. Installing standalone OpenWrt binaries..."
        local bin_arch
        case "$HOST_ARCH" in
            x86_64) bin_arch="amd64" ;;
            aarch64) bin_arch="arm64" ;;
            riscv64) bin_arch="riscv64" ;;
            *) abort "No precompiled standalone tarball available for architecture: ${HOST_ARCH}" ;;
        esac

        local tar_name="gsm2mqtt-${TAG}-linux-${bin_arch}.tar.gz"
        local tar_url="https://github.com/${REPO}/releases/download/${TAG}/${tar_name}"
        info "Downloading ${tar_name}..."
        fetch "$tar_url" "${TMP_DIR}/archive.tar.gz"
        tar -xzf "${TMP_DIR}/archive.tar.gz" -C "${TMP_DIR}"

        mkdir -p /usr/bin /etc/gsm2mqtt /etc/config /etc/init.d
        cp -f "${TMP_DIR}/gsm2mqtt" /usr/bin/gsm2mqtt
        chmod 755 /usr/bin/gsm2mqtt

        if [ ! -f /etc/gsm2mqtt/gsm2mqtt.yaml ]; then
            fetch "https://raw.githubusercontent.com/${REPO}/${TAG}/deployments/openwrt/files/gsm2mqtt.yaml" /etc/gsm2mqtt/gsm2mqtt.yaml
            chmod 640 /etc/gsm2mqtt/gsm2mqtt.yaml
        fi

        if [ ! -f /etc/config/gsm2mqtt ]; then
            fetch "https://raw.githubusercontent.com/${REPO}/${TAG}/deployments/openwrt/files/gsm2mqtt.config" /etc/config/gsm2mqtt
            chmod 644 /etc/config/gsm2mqtt
        fi

        fetch "https://raw.githubusercontent.com/${REPO}/${TAG}/deployments/openwrt/files/gsm2mqtt.init" /etc/init.d/gsm2mqtt
        chmod 755 /etc/init.d/gsm2mqtt
        /etc/init.d/gsm2mqtt enable || true
    fi

    success "GSM2MQTT installed successfully on OpenWrt!"
    echo
    printf "${BOLD}Next steps for OpenWrt:${NC}\n"
    printf "  1. Edit configuration:  ${YELLOW}vi /etc/gsm2mqtt/gsm2mqtt.yaml${NC}\n"
    printf "  2. Start service:       ${GREEN}/etc/init.d/gsm2mqtt start${NC}\n"
    printf "  3. View live logs:      ${BLUE}logread -f -e gsm2mqtt${NC}\n"
}

# ------------------------------------------------------------------------------
# Installation on Linux with systemd
# ------------------------------------------------------------------------------
install_systemd() {
    local bin_arch
    case "$HOST_ARCH" in
        x86_64) bin_arch="amd64" ;;
        aarch64|arm64) bin_arch="arm64" ;;
        riscv64) bin_arch="riscv64" ;;
        *) abort "Unsupported systemd architecture: ${HOST_ARCH}. Supported: x86_64, aarch64, riscv64" ;;
    esac

    local tar_name="gsm2mqtt-${TAG}-linux-${bin_arch}.tar.gz"
    local tar_url="https://github.com/${REPO}/releases/download/${TAG}/${tar_name}"

    info "Downloading ${tar_name}..."
    fetch "$tar_url" "${TMP_DIR}/archive.tar.gz"
    tar -xzf "${TMP_DIR}/archive.tar.gz" -C "${TMP_DIR}"

    local bin_dest="${BIN_DIR:-/usr/local/bin}"
    mkdir -p "$bin_dest" /etc/gsm2mqtt
    cp -f "${TMP_DIR}/gsm2mqtt" "${bin_dest}/gsm2mqtt"
    chmod 755 "${bin_dest}/gsm2mqtt"
    success "Installed executable to ${bin_dest}/gsm2mqtt"

    # Install default config if not already present
    if [ -f /etc/gsm2mqtt/gsm2mqtt.yaml ]; then
        info "Preserving existing configuration at /etc/gsm2mqtt/gsm2mqtt.yaml"
    else
        cp "${TMP_DIR}/gsm2mqtt.example.yaml" /etc/gsm2mqtt/gsm2mqtt.yaml
        chmod 640 /etc/gsm2mqtt/gsm2mqtt.yaml
        success "Created initial configuration at /etc/gsm2mqtt/gsm2mqtt.yaml"
    fi

    # Create service user and ensure dialout access
    local dial_grp="dialout"
    if ! getent group dialout >/dev/null 2>&1; then
        if getent group uucp >/dev/null 2>&1; then
            dial_grp="uucp"
        fi
    fi

    if ! id -u gsm2mqtt >/dev/null 2>&1; then
        info "Creating system user 'gsm2mqtt' with '${dial_grp}' access..."
        useradd -r -M -s /bin/false -G "$dial_grp" gsm2mqtt 2>/dev/null || useradd -r -M -s /bin/false gsm2mqtt 2>/dev/null || true
    fi

    chown -R root:"$dial_grp" /etc/gsm2mqtt 2>/dev/null || true
    chmod 750 /etc/gsm2mqtt

    # Install systemd unit
    info "Installing systemd unit service..."
    fetch "https://raw.githubusercontent.com/${REPO}/${TAG}/deployments/systemd/gsm2mqtt.service" /etc/systemd/system/gsm2mqtt.service
    chmod 644 /etc/systemd/system/gsm2mqtt.service

    systemctl daemon-reload
    systemctl enable gsm2mqtt.service || true
    success "systemd service registered and enabled (gsm2mqtt.service)"

    # Detect connected modems
    local found_modems
    found_modems="$(ls -d /dev/ttyUSB* /dev/ttyACM* 2>/dev/null || true)"

    echo
    printf "${BOLD}Next steps:${NC}\n"
    if [ -n "$found_modems" ]; then
        printf "  ${GREEN}Detected modem devices:${NC} %s\n" "$found_modems"
    else
        printf "  ${YELLOW}Note:${NC} No /dev/ttyUSB* or /dev/ttyACM* modem ports detected yet.\n"
    fi
    printf "  1. Configure your broker and modems:  ${YELLOW}nano /etc/gsm2mqtt/gsm2mqtt.yaml${NC}\n"
    printf "  2. Start GSM2MQTT service:           ${GREEN}systemctl start gsm2mqtt${NC}\n"
    printf "  3. Check service status:              ${BLUE}systemctl status gsm2mqtt${NC}\n"
    printf "  4. View live logs:                   ${BLUE}journalctl -u gsm2mqtt -f${NC}\n"
}

# ------------------------------------------------------------------------------
# Installation on Generic / Other Linux
# ------------------------------------------------------------------------------
install_generic() {
    local bin_arch
    case "$HOST_ARCH" in
        x86_64) bin_arch="amd64" ;;
        aarch64|arm64) bin_arch="arm64" ;;
        riscv64) bin_arch="riscv64" ;;
        *) abort "Unsupported architecture: ${HOST_ARCH}." ;;
    esac

    local tar_name="gsm2mqtt-${TAG}-linux-${bin_arch}.tar.gz"
    local tar_url="https://github.com/${REPO}/releases/download/${TAG}/${tar_name}"

    info "Downloading ${tar_name}..."
    fetch "$tar_url" "${TMP_DIR}/archive.tar.gz"
    tar -xzf "${TMP_DIR}/archive.tar.gz" -C "${TMP_DIR}"

    local bin_dest="${BIN_DIR:-/usr/local/bin}"
    mkdir -p "$bin_dest" /etc/gsm2mqtt
    cp -f "${TMP_DIR}/gsm2mqtt" "${bin_dest}/gsm2mqtt"
    chmod 755 "${bin_dest}/gsm2mqtt"

    if [ ! -f /etc/gsm2mqtt/gsm2mqtt.yaml ]; then
        cp "${TMP_DIR}/gsm2mqtt.example.yaml" /etc/gsm2mqtt/gsm2mqtt.yaml
        chmod 640 /etc/gsm2mqtt/gsm2mqtt.yaml
    fi

    success "Installed executable to ${bin_dest}/gsm2mqtt"
    printf "Run GSM2MQTT with: ${GREEN}%s/gsm2mqtt --config /etc/gsm2mqtt/gsm2mqtt.yaml${NC}\n" "$bin_dest"
}

# Dispatch
case "$SYS_TYPE" in
    openwrt)
        install_openwrt
        ;;
    systemd)
        install_systemd
        ;;
    *)
        install_generic
        ;;
esac
