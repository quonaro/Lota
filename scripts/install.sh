#!/bin/sh
# Install script for Lota CLI tool
# Supports Linux and macOS

set -e

# Detect if colors are supported
if [ -t 1 ] && command -v tput > /dev/null 2>&1; then
    RED=$(tput setaf 1)
    GREEN=$(tput setaf 2)
    YELLOW=$(tput setaf 3)
    BLUE=$(tput setaf 4)
    BOLD=$(tput bold)
    RESET=$(tput sgr0)
    HAS_COLORS=1
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    BOLD=''
    RESET=''
    HAS_COLORS=0
fi

# Detect Unicode support
if [ "$HAS_COLORS" = "1" ] && [ -n "$LANG" ] && echo "$LANG" | grep -q "UTF-8\|utf8"; then
    CHECK="${GREEN}✓${RESET}"
    CROSS="${RED}✗${RESET}"
    ARROW="${BLUE}→${RESET}"
    INFO="${BLUE}ℹ${RESET}"
    WARN="${YELLOW}⚠${RESET}"
    LOTA_ICON="⚡"
else
    CHECK="${GREEN}[OK]${RESET}"
    CROSS="${RED}[FAIL]${RESET}"
    ARROW="${BLUE}=>${RESET}"
    INFO="${BLUE}[i]${RESET}"
    WARN="${YELLOW}[!]${RESET}"
    LOTA_ICON="Lota"
fi

# Initialize variables with defaults
REPO="quonaro/lota"
VERSION="latest"
INSTALL_SCOPE="user"
INTERACTIVE=0

# Check if running interactively
if [ -t 0 ]; then
    INTERACTIVE=1
    INPUT_SOURCE="/dev/stdin"
elif [ -r /dev/tty ]; then
    INTERACTIVE=1
    INPUT_SOURCE="/dev/tty"
fi

# Parse command line arguments
VERSION_EXPLICITLY_SET=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        -V|--version)
            if [ -n "$2" ]; then
                VERSION="$2"
                VERSION_EXPLICITLY_SET=1
                shift 2
            else
                echo "${CROSS} ${BOLD}${RED}Error: Argument for $1 is missing${RESET}" >&2
                exit 1
            fi
            ;;
        -h|--help)
            echo "Usage: ./install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -V, --version <ver>     Install specific version (default: latest)"
            echo "  -h, --help              Show this help message"
            echo ""
            exit 0
            ;;
        *)
            echo "${CROSS} ${BOLD}${RED}Error: Unknown option $1${RESET}" >&2
            echo "   ${INFO} Run with --help for usage information" >&2
            exit 1
            ;;
    esac
done

# Apply environment variables (CLI args take precedence via explicit set flags)
VERSION="${LOTA_VERSION:-$VERSION}"
INSTALL_SCOPE="${LOTA_INSTALL_SCOPE:-$INSTALL_SCOPE}"

# Platform detection
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux*)     PLATFORM="linux" ;;
    Darwin*)    PLATFORM="macos" ;;
    *)          echo "${CROSS} ${BOLD}${RED}Error: Unsupported OS: $OS${RESET}" >&2; exit 1 ;;
esac

case "$ARCH" in
    x86_64|amd64) ARCHITECTURE="amd64" ;;
    aarch64|arm64) ARCHITECTURE="arm64" ;;
    *)          echo "${CROSS} ${BOLD}${RED}Error: Unsupported architecture: $ARCH${RESET}" >&2; exit 1 ;;
esac

# Determine binary name
BINARY_NAME="lota"

# Interactive Prompts
if [ "$INTERACTIVE" = "1" ] && [ -z "$LOTA_NONINTERACTIVE" ]; then
    # Version Prompt
    if [ "$VERSION_EXPLICITLY_SET" != "1" ] && [ "$VERSION" = "latest" ]; then
        echo ""
        printf "${INFO} Do you want to install a specific version? [y/N] "
        read -r REPLY < "$INPUT_SOURCE"
        if echo "$REPLY" | grep -iq "^y"; then
            echo "${INFO} Fetching recent versions..."
            if command -v curl > /dev/null 2>&1; then
                API_RESPONSE=$(curl -s -w "\n%{http_code}" "https://api.github.com/repos/${REPO}/releases?per_page=20")
                HTTP_CODE=$(echo "$API_RESPONSE" | tail -n1)
                BODY=$(echo "$API_RESPONSE" | sed '$d')

                if [ "$HTTP_CODE" = "403" ] || [ "$HTTP_CODE" = "429" ]; then
                    echo "${WARN} GitHub API rate limit exceeded (HTTP $HTTP_CODE)."
                    echo "${INFO} You can still enter a version manually."
                    printf "${INFO} Enter version (e.g., v1.2.3): "
                    read -r V_INPUT < "$INPUT_SOURCE"
                    if [ -n "$V_INPUT" ]; then
                        CLEAN_VERSION="${V_INPUT#v}"
                        VERSION="$CLEAN_VERSION"
                        echo "   ${ARROW} You selected: ${BOLD}v${VERSION}${RESET}"
                    fi
                elif [ -n "$BODY" ]; then
                    TAGS=$(echo "$BODY" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
                fi

                if [ -n "$TAGS" ]; then
                    echo "${INFO} Available versions:"

                    TOTAL_LINES=$(echo "$TAGS" | wc -l)
                    CURRENT_LINE=1
                    PAGE_SIZE=5

                    while true; do
                        END_LINE=$((CURRENT_LINE + PAGE_SIZE - 1))

                        echo "$TAGS" | sed -n "${CURRENT_LINE},${END_LINE}p" | while read -r line; do
                            echo "   - ${BOLD}$line${RESET}"
                        done

                        REMAINING=$((TOTAL_LINES - END_LINE))

                        echo ""
                        if [ "$REMAINING" -gt 0 ]; then
                            printf "${INFO} Enter version (or press 'm' to see more): "
                        else
                             printf "${INFO} Enter version: "
                        fi

                        read -r V_INPUT < "$INPUT_SOURCE"

                        if [ "$V_INPUT" = "m" ] && [ "$REMAINING" -gt 0 ]; then
                            CURRENT_LINE=$((END_LINE + 1))
                            continue
                        elif [ -n "$V_INPUT" ]; then
                            CLEAN_VERSION="${V_INPUT#v}"
                            VERSION="$CLEAN_VERSION"
                            echo "   ${ARROW} You selected: ${BOLD}v${VERSION}${RESET}"
                            break
                        fi
                    done
                else
                    echo "${WARN} Could not fetch versions (network or API limit). You can still type a version manually."
                    printf "${INFO} Enter version: "
                    read -r V_INPUT < "$INPUT_SOURCE"
                    if [ -n "$V_INPUT" ]; then
                        CLEAN_VERSION="${V_INPUT#v}"
                        VERSION="$CLEAN_VERSION"
                        echo "   ${ARROW} You selected: ${BOLD}v${VERSION}${RESET}"
                    fi
                fi
            else
                echo "${WARN} curl not found, cannot list versions. Please type manually."
                printf "${INFO} Enter version: "
                read -r V_INPUT < "$INPUT_SOURCE"
                if [ -n "$V_INPUT" ]; then
                    CLEAN_VERSION="${V_INPUT#v}"
                    VERSION="$CLEAN_VERSION"
                    echo "   ${ARROW} You selected: ${BOLD}v${VERSION}${RESET}"
                fi
            fi
        fi
    fi

    # Install Path Prompt
    echo ""
    echo "${INFO} ${BOLD}Choose installation location:${RESET}"
    echo "   1) User   (${HOME}/.local/bin) [Default]"

    if [ "$PLATFORM" = "macos" ]; then
        SYSTEM_PATH="/usr/local/bin"
    else
        SYSTEM_PATH="/usr/local/bin"
    fi

    echo "   2) System (${SYSTEM_PATH})"
    echo "   3) Custom"

    printf "${INFO} Enter selection [1]: "
    read -r REPLY < "$INPUT_SOURCE"

    case "$REPLY" in
        2)
            INSTALL_DIR="${SYSTEM_PATH}"
            INSTALL_SCOPE="system"
            ;;
        3)
            printf "${INFO} Enter custom path: "
            read -r CUSTOM_PATH < "$INPUT_SOURCE"
            CUSTOM_PATH="${CUSTOM_PATH/#\~/$HOME}"
            if [ -z "$CUSTOM_PATH" ]; then
                 echo "${WARN} No path entered, defaulting to User location.${RESET}"
                 INSTALL_DIR="${HOME}/.local/bin"
                 INSTALL_SCOPE="user"
            else
                 INSTALL_DIR="$CUSTOM_PATH"
                 INSTALL_SCOPE="custom"
            fi
            ;;
        *)
            INSTALL_DIR="${HOME}/.local/bin"
            INSTALL_SCOPE="user"
            ;;
    esac
    echo "   ${ARROW} Installing to ${BOLD}${INSTALL_DIR}${RESET}"
fi

# Set defaults if not set interactively (for non-interactive mode)
if [ -z "$INSTALL_DIR" ]; then
    case "${INSTALL_SCOPE}" in
        global|system)
            INSTALL_DIR="/usr/local/bin"
            ;;
        user|"")
            INSTALL_DIR="${HOME}/.local/bin"
            ;;
        *)
             echo "${WARN} Unknown LOTA_INSTALL_SCOPE='${INSTALL_SCOPE}', falling back to user scope${RESET}" >&2
             INSTALL_DIR="${HOME}/.local/bin"
             INSTALL_SCOPE="user"
            ;;
    esac
fi

BINARY_PATH="${INSTALL_DIR}/${BINARY_NAME}"

# Print header
echo ""
if [ "$HAS_COLORS" = "1" ]; then
    echo "${BOLD}${BLUE}╔════════════════════════════════════════╗${RESET}"
    echo "${BOLD}${BLUE}║${RESET}  ${BOLD}${LOTA_ICON} Lota CLI Installer${RESET}              ${BOLD}${BLUE}║${RESET}"
    echo "${BOLD}${BLUE}╚════════════════════════════════════════╝${RESET}"
else
    echo "${BOLD}${LOTA_ICON} Lota CLI Installer${RESET}"
    echo "========================================"
fi
echo ""

# Print system information
echo "${INFO} ${BOLD}Detected system:${RESET}"
echo "   ${ARROW} Platform: ${BOLD}${PLATFORM}-${ARCHITECTURE}${RESET}"
echo "   ${ARROW} Install scope: ${BOLD}${INSTALL_SCOPE}${RESET}"
echo "   ${ARROW} Install directory: ${BOLD}${INSTALL_DIR}${RESET}"
echo ""

# Download binary
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/lota-${PLATFORM}-${ARCHITECTURE}"
else
    URL="https://github.com/${REPO}/releases/download/v${VERSION}/lota-${PLATFORM}-${ARCHITECTURE}"
fi

# Add .exe for Windows
if [ "$PLATFORM" = "windows" ]; then
    URL="${URL}.exe"
fi

# Download with curl or wget
TEMP_DIR=$(mktemp -d)
TEMP_FILE="${TEMP_DIR}/lota-${PLATFORM}-${ARCHITECTURE}"
if [ "$PLATFORM" = "windows" ]; then
    TEMP_FILE="${TEMP_FILE}.exe"
fi
CHECKSUM_FILE="${TEMP_DIR}/checksums.txt"

echo ""
echo "${INFO} ${BOLD}Downloading Lota CLI...${RESET}"
echo "   ${ARROW} ${URL}"

# Download checksums file
if [ "$VERSION" = "latest" ]; then
    CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/checksums.txt"
else
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt"
fi

echo "   ${ARROW} ${CHECKSUM_URL}"
if command -v curl > /dev/null 2>&1; then
    curl -L -o "${CHECKSUM_FILE}" "${CHECKSUM_URL}" -s -S --show-error 2>/dev/null || true
elif command -v wget > /dev/null 2>&1; then
    wget -O "${CHECKSUM_FILE}" "${CHECKSUM_URL}" 2>/dev/null || true
fi

# Download and check for errors
if command -v curl > /dev/null 2>&1; then
    HTTP_CODE=$(curl -L -w "%{http_code}" -o "${TEMP_FILE}" "${URL}" -s -S --show-error)
    if [ "$HTTP_CODE" != "200" ]; then
        echo ""
        echo "${CROSS} ${BOLD}${RED}Error: Failed to download binary (HTTP $HTTP_CODE)${RESET}" >&2
        echo "   ${WARN} The release may not exist yet. Please check:" >&2
        echo "      https://github.com/${REPO}/releases" >&2
        rm -rf "${TEMP_DIR}"
        exit 1
    fi
    echo "   ${CHECK} Download completed"
elif command -v wget > /dev/null 2>&1; then
    if ! wget -O "${TEMP_FILE}" "${URL}" 2>&1 | grep -q "200 OK"; then
        echo ""
        echo "${CROSS} ${BOLD}${RED}Error: Failed to download binary${RESET}" >&2
        echo "   ${WARN} The release may not exist yet. Please check:" >&2
        echo "      https://github.com/${REPO}/releases" >&2
        rm -rf "${TEMP_DIR}"
        exit 1
    fi
    echo "   ${CHECK} Download completed"
else
    echo ""
    echo "${CROSS} ${BOLD}${RED}Error: Neither curl nor wget found${RESET}" >&2
    echo "   ${WARN} Please install curl or wget to continue" >&2
    exit 1
fi

# Verify downloaded file is valid
echo "${INFO} ${BOLD}Verifying download...${RESET}"
if [ ! -s "${TEMP_FILE}" ]; then
    echo ""
    echo "${CROSS} ${BOLD}${RED}Error: Downloaded file is empty${RESET}" >&2
    echo "   ${WARN} The release may not exist yet. Please check:" >&2
    echo "      https://github.com/${REPO}/releases" >&2
    rm -rf "${TEMP_DIR}"
    exit 1
fi
echo "   ${CHECK} Download verified"

# Verify checksum if available
if [ -s "${CHECKSUM_FILE}" ]; then
    echo "${INFO} ${BOLD}Verifying checksum...${RESET}"
    BINARY_NAME="lota-${PLATFORM}-${ARCHITECTURE}"
    if [ "$PLATFORM" = "windows" ]; then
        BINARY_NAME="${BINARY_NAME}.exe"
    fi
    EXPECTED_CHECKSUM=$(grep "${BINARY_NAME}" "${CHECKSUM_FILE}" 2>/dev/null | awk '{print $1}')
    if [ -n "$EXPECTED_CHECKSUM" ] && command -v sha256sum > /dev/null 2>&1; then
        ACTUAL_CHECKSUM=$(sha256sum "${TEMP_FILE}" | awk '{print $1}')
        if [ "$EXPECTED_CHECKSUM" = "$ACTUAL_CHECKSUM" ]; then
            echo "   ${CHECK} Checksum verified (SHA-256)"
        else
            echo "${CROSS} ${BOLD}${RED}Error: Checksum verification failed!${RESET}" >&2
            echo "   Expected: ${EXPECTED_CHECKSUM}" >&2
            echo "   Actual:   ${ACTUAL_CHECKSUM}" >&2
            rm -rf "${TEMP_DIR}"
            exit 1
        fi
    else
        echo "   ${WARN} Could not verify checksum (sha256sum not available or checksum not found)"
    fi
fi

# Check write permissions and configure sudo if needed (after verification, before install)
SUDO=""
if [ -d "$INSTALL_DIR" ]; then
    if [ ! -w "$INSTALL_DIR" ]; then
        if command -v sudo >/dev/null 2>&1; then
            SUDO="sudo"
            echo "${INFO} ${BOLD}Note: Installation to ${INSTALL_DIR} requires sudo privileges.${RESET}"
        else
            echo "${CROSS} ${BOLD}${RED}Error: ${INSTALL_DIR} is not writable and sudo is not available.${RESET}" >&2
            rm -rf "${TEMP_DIR}"
            exit 1
        fi
    fi
else
     PARENT_DIR=$(dirname "$INSTALL_DIR")
     if [ -d "$PARENT_DIR" ] && [ ! -w "$PARENT_DIR" ]; then
        if command -v sudo >/dev/null 2>&1; then
            SUDO="sudo"
            echo "${INFO} ${BOLD}Note: Creating ${INSTALL_DIR} requires sudo privileges.${RESET}"
        else
            echo "${CROSS} ${BOLD}${RED}Error: Cannot create ${INSTALL_DIR} (permission denied) and sudo is not available.${RESET}" >&2
            rm -rf "${TEMP_DIR}"
            exit 1
        fi
     fi
fi

# Create install directory if it doesn't exist
echo "${INFO} ${BOLD}Installing binary...${RESET}"
$SUDO mkdir -p "${INSTALL_DIR}"

# Install binary
$SUDO mv "${TEMP_FILE}" "${BINARY_PATH}"
$SUDO chmod +x "${BINARY_PATH}"
echo "   ${CHECK} Binary installed to ${BINARY_PATH}"

if [ "$VERSION" != "latest" ]; then
    echo "   ${CHECK} Installed version: ${BOLD}${VERSION}${RESET}"
fi

# Cleanup
rm -rf "${TEMP_DIR}"

# Check if ~/.local/bin is in PATH and add to all existing shell configs
echo ""
PATH_EXPORT="export PATH=\"\${HOME}/.local/bin:\${PATH}\""

if ! echo "${PATH}" | grep -Eq "(^|:)${HOME}/\.local/bin(:|$)"; then
    echo "${INFO} ${BOLD}Configuring PATH...${RESET}"

    export PATH="${HOME}/.local/bin:${PATH}"
    echo "   ${CHECK} Added to PATH for current session"

    ADDED_TO_CONFIG=0
    CONFIG_FILES_ADDED=()
    RELOAD_CMDS=()

    # Check and add to .zshrc
    if [ -f "${HOME}/.zshrc" ]; then
        if ! grep -Eq "(PATH.*)${HOME}/\.local/bin" "${HOME}/.zshrc" 2>/dev/null; then
            echo "" >> "${HOME}/.zshrc"
            echo "# Added by Lota CLI installer" >> "${HOME}/.zshrc"
            echo "${PATH_EXPORT}" >> "${HOME}/.zshrc"
            echo "   ${CHECK} Added to ~/.zshrc"
            CONFIG_FILES_ADDED+=("~/.zshrc")
            RELOAD_CMDS+=("source ~/.zshrc")
            ADDED_TO_CONFIG=1
        else
            echo "   ${CHECK} Already in ~/.zshrc"
            ADDED_TO_CONFIG=1
        fi
    fi

    # Check and add to .bashrc
    if [ -f "${HOME}/.bashrc" ]; then
        if ! grep -Eq "(PATH.*)${HOME}/\.local/bin" "${HOME}/.bashrc" 2>/dev/null; then
            echo "" >> "${HOME}/.bashrc"
            echo "# Added by Lota CLI installer" >> "${HOME}/.bashrc"
            echo "${PATH_EXPORT}" >> "${HOME}/.bashrc"
            echo "   ${CHECK} Added to ~/.bashrc"
            CONFIG_FILES_ADDED+=("~/.bashrc")
            RELOAD_CMDS+=("source ~/.bashrc")
            ADDED_TO_CONFIG=1
        else
            echo "   ${CHECK} Already in ~/.bashrc"
            ADDED_TO_CONFIG=1
        fi
    fi

    # Check and add to .bash_profile
    if [ -f "${HOME}/.bash_profile" ]; then
        if ! grep -Eq "(PATH.*)${HOME}/\.local/bin" "${HOME}/.bash_profile" 2>/dev/null; then
            echo "" >> "${HOME}/.bash_profile"
            echo "# Added by Lota CLI installer" >> "${HOME}/.bash_profile"
            echo "${PATH_EXPORT}" >> "${HOME}/.bash_profile"
            echo "   ${CHECK} Added to ~/.bash_profile"
            CONFIG_FILES_ADDED+=("~/.bash_profile")
            RELOAD_CMDS+=("source ~/.bash_profile")
            ADDED_TO_CONFIG=1
        else
            echo "   ${CHECK} Already in ~/.bash_profile"
            ADDED_TO_CONFIG=1
        fi
    fi

    # Check and add to fish config
    if command -v fish > /dev/null 2>&1; then
        FISH_CONFIG_DIR="${HOME}/.config/fish"
        FISH_CONFIG_FILE="${FISH_CONFIG_DIR}/config.fish"
        if [ -d "${FISH_CONFIG_DIR}" ] || mkdir -p "${FISH_CONFIG_DIR}" 2>/dev/null; then
            if [ ! -f "${FISH_CONFIG_FILE}" ]; then
                touch "${FISH_CONFIG_FILE}"
            fi
            if ! grep -Eq "PATH.*${HOME}/\.local/bin" "${FISH_CONFIG_FILE}" 2>/dev/null; then
                echo "" >> "${FISH_CONFIG_FILE}"
                echo "# Added by Lota CLI installer" >> "${FISH_CONFIG_FILE}"
                echo "set -gx PATH \"\${HOME}/.local/bin\" \$PATH" >> "${FISH_CONFIG_FILE}"
                echo "   ${CHECK} Added to ~/.config/fish/config.fish"
                CONFIG_FILES_ADDED+=("~/.config/fish/config.fish")
                RELOAD_CMDS+=("source ~/.config/fish/config.fish")
                ADDED_TO_CONFIG=1
            else
                echo "   ${CHECK} Already in ~/.config/fish/config.fish"
                ADDED_TO_CONFIG=1
            fi
        fi
    fi

    # Check and add to .profile as fallback
    if [ $ADDED_TO_CONFIG -eq 0 ]; then
        PROFILE_FILE="${HOME}/.profile"
        if [ -f "${PROFILE_FILE}" ]; then
            if ! grep -Eq "(PATH.*)${HOME}/\.local/bin" "${PROFILE_FILE}" 2>/dev/null; then
                echo "" >> "${PROFILE_FILE}"
                echo "# Added by Lota CLI installer" >> "${PROFILE_FILE}"
                echo "${PATH_EXPORT}" >> "${PROFILE_FILE}"
                echo "   ${CHECK} Added to ~/.profile"
                CONFIG_FILES_ADDED+=("~/.profile")
                RELOAD_CMDS+=("source ~/.profile")
                ADDED_TO_CONFIG=1
            else
                echo "   ${CHECK} Already in ~/.profile"
                ADDED_TO_CONFIG=1
            fi
        else
            echo "${PATH_EXPORT}" > "${PROFILE_FILE}"
            chmod 644 "${PROFILE_FILE}"
            echo "   ${CHECK} Created ~/.profile with PATH"
            CONFIG_FILES_ADDED+=("~/.profile")
            RELOAD_CMDS+=("source ~/.profile")
            ADDED_TO_CONFIG=1
        fi
    fi

    if [ $ADDED_TO_CONFIG -eq 1 ]; then
        echo ""
        if [ ${#CONFIG_FILES_ADDED[@]} -gt 0 ]; then
            echo "   ${INFO} PATH has been added to:"
            for config_file in "${CONFIG_FILES_ADDED[@]}"; do
                echo "      ${BOLD}${config_file}${RESET}"
            done
        fi
        echo ""
        echo "   ${INFO} Run one of these commands to reload your shell configuration:"
        for reload_cmd in "${RELOAD_CMDS[@]}"; do
            echo "      ${BOLD}${reload_cmd}${RESET}"
        done
        echo "   ${INFO} Or simply restart your terminal."
    else
        echo ""
        echo "   ${WARN} Could not automatically add to shell config."
        echo "   ${WARN} Please add this line manually to your shell configuration:"
        echo "   ${BOLD}${GREEN}${PATH_EXPORT}${RESET}"
    fi
    echo ""
else
    echo "${CHECK} ${BOLD}${GREEN}Already in PATH${RESET}"
    echo ""
fi

# Success message
echo ""
if [ "$HAS_COLORS" = "1" ]; then
    echo "${BOLD}${GREEN}╔════════════════════════════════════════╗${RESET}"
    echo "${BOLD}${GREEN}║${RESET}  ${CHECK} ${BOLD}Lota CLI installed successfully!${RESET}  ${BOLD}${GREEN}║${RESET}"
    echo "${BOLD}${GREEN}╚════════════════════════════════════════╝${RESET}"
else
    echo "${CHECK} ${BOLD}Lota CLI installed successfully!${RESET}"
    echo "========================================"
fi
echo ""
echo "   Run ${BOLD}lota --version${RESET} to verify installation."
echo ""
