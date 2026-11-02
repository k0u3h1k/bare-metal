#!/bin/sh
# Unbound installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/k0u3h1k/bare-metal/main/scripts/install.sh | sh
set -eu

VERSION="v0.2.0"
REPO="k0u3h1k/bare-metal"
BIN="unbound"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

# checksums
SUM_LINUX_AMD64="aa8cadabfa648f481dc2ef6c71f52ab244179737feb7d732e9ff523ef68ab439"
SUM_DARWIN_AMD64="a0adab3763213537a2f34a59475cc3b3095028984877a6d33a4e9a932a66ece7"
SUM_DARWIN_ARM64="cd56988cf671ad31d39d49375cc027bda62a088a401dd5890ed8fb9d731dd905"

R='\033[0;31m'; G='\033[0;32m'; Y='\033[1;33m'; C='\033[0;36m'; B='\033[1m'; N='\033[0m'

usage() {
    cat <<EOU
Unbound Installer ${VERSION}

USAGE:
    curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sh
    curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sh -s -- [FLAGS]

FLAGS:
    --help          Show this help
    --version       Print version
    --bin-dir=DIR   Custom install directory (default: /usr/local/bin or ~/.local/bin)
EOU
    exit 0
}

version() { echo "Unbound installer ${VERSION}"; exit 0; }

BIN_DIR=""
for arg in "$@"; do
    case "$arg" in
        --help) usage ;;
        --version) version ;;
        --bin-dir=*) BIN_DIR="${arg#*=}" ;;
        *) echo "Unknown: $arg"; usage ;;
    esac
done

detect_platform() {
    local os arch
    case "$(uname -s)" in Linux) os="linux" ;; Darwin) os="darwin" ;;
        *) echo "Unsupported OS"; exit 1 ;; esac
    case "$(uname -m)" in x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) [ "$os" = "linux" ] && { echo "Linux ARM64 unsupported"; exit 1; }; arch="arm64" ;;
        *) echo "Unsupported arch"; exit 1 ;; esac
    echo "${os}-${arch}"
}

detect_bin_dir() {
    [ -n "$BIN_DIR" ] && { echo "$BIN_DIR"; return; }
    if [ "$(id -u)" -eq 0 ] || [ -w "/usr/local/bin" ]; then echo "/usr/local/bin"
    else echo "${HOME}/.local/bin"; fi
}

main() {
    echo "${C}${B}Unbound ${VERSION} Installer${N}"; echo ""
    local plat; plat=$(detect_platform)
    local dir; dir=$(detect_bin_dir)
    local url="${BASE}/${BIN}-${plat}"
    local dest="${dir}/${BIN}"
    echo "Platform: ${plat}"; echo "Install to: ${dir}"; echo ""

    [ -d "$dir" ] || mkdir -p "$dir" 2>/dev/null || sudo mkdir -p "$dir" 2>/dev/null || { echo "Cannot create ${dir}"; exit 1; }

    local tmp; tmp=$(mktemp); trap 'rm -f "${tmp:-}"' EXIT
    echo "Downloading ${BIN} ${VERSION}..."
    if command -v curl >/dev/null 2>&1; then curl -fsSL "$url" -o "$tmp"
    elif command -v wget >/dev/null 2>&1; then wget -q "$url" -O "$tmp"
    else echo "Need curl or wget"; exit 1; fi

    echo "Verifying SHA256 checksum..."
    local sum=""
    if command -v shasum >/dev/null 2>&1; then sum=$(shasum -a 256 "$tmp" | cut -d' ' -f1)
    elif command -v sha256sum >/dev/null 2>&1; then sum=$(sha256sum "$tmp" | cut -d' ' -f1)
    else echo "No SHA256 tool found"; rm -f "$tmp"; exit 1; fi

    local expected=""
    case "$plat" in linux-amd64) expected="$SUM_LINUX_AMD64" ;;
        darwin-amd64) expected="$SUM_DARWIN_AMD64" ;;
        darwin-arm64) expected="$SUM_DARWIN_ARM64" ;; esac

    [ "$sum" != "$expected" ] && { echo "SHA256 mismatch"; echo "  Expected: $expected"; echo "  Got: $sum"; rm -f "$tmp"; exit 1; }
    echo "Checksum verified OK"

    if [ -w "$dir" ]; then install -m 755 "$tmp" "$dest"
    else sudo install -m 755 "$tmp" "$dest"; fi

    echo ""
    echo "${G}${B}Unbound ${VERSION} installed!${N}"
    case ":${PATH}:" in *":${dir}:"*) ;; *)
        echo "${Y}Warning: ${dir} not in PATH${N}"
        echo "Add: export PATH=\"${dir}:\$PATH\""
    ;; esac
    echo ""
    echo "Quick start:"
    echo "  unbound --help"
    echo "  unbound pull llama3.2:1b"
    echo "  unbound run llama3.2:1b"
}

main "$@"