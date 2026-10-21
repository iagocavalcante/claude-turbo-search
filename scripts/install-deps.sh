#!/bin/bash
# install-deps.sh - Install dependencies for claude-turbo-search
# Usage: ./install-deps.sh [--check-only]
# Supports: macOS (Homebrew), Linux (apt, dnf, pacman)

set -e

CHECK_ONLY=false
if [ "$1" = "--check-only" ]; then
  CHECK_ONLY=true
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS and package manager
detect_os() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "macos"
  elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "linux"
  else
    echo "unknown"
  fi
}

detect_package_manager() {
  if command -v brew &> /dev/null; then
    echo "brew"
  elif command -v apt-get &> /dev/null; then
    echo "apt"
  elif command -v dnf &> /dev/null; then
    echo "dnf"
  elif command -v pacman &> /dev/null; then
    echo "pacman"
  else
    echo "unknown"
  fi
}

OS=$(detect_os)
PKG_MANAGER=$(detect_package_manager)

check_command() {
  local cmd=$1
  local name=$2
  if command -v "$cmd" &> /dev/null; then
    local version=$("$cmd" --version 2>/dev/null | head -1 || echo "installed")
    echo -e "  ${GREEN}✓${NC} $name ($version)"
    return 0
  else
    echo -e "  ${RED}✗${NC} $name - not installed"
    return 1
  fi
}

install_package() {
  local pkg_brew=$1
  local pkg_apt=$2
  local pkg_dnf=$3
  local pkg_pacman=$4
  local tap=$5

  case $PKG_MANAGER in
    brew)
      if [ -n "$tap" ]; then
        echo "  Adding tap: $tap"
        brew tap "$tap" 2>/dev/null || true
      fi
      echo "  Installing $pkg_brew..."
      brew install "$pkg_brew"
      ;;
    apt)
      echo "  Installing $pkg_apt..."
      sudo apt-get update -qq
      sudo apt-get install -y "$pkg_apt"
      ;;
    dnf)
      echo "  Installing $pkg_dnf..."
      sudo dnf install -y "$pkg_dnf"
      ;;
    pacman)
      echo "  Installing $pkg_pacman..."
      sudo pacman -S --noconfirm "$pkg_pacman"
      ;;
    *)
      echo -e "${RED}Error: No supported package manager found.${NC}"
      echo "Please install manually:"
      echo "  - ripgrep: https://github.com/BurntSushi/ripgrep"
      echo "  - fzf: https://github.com/junegunn/fzf"
      echo "  - jq: https://github.com/stedolan/jq"
      echo "  - bun: https://bun.sh"
      echo "  - qmd: https://github.com/tobi/qmd"
      exit 1
      ;;
  esac
}

install_bun() {
  case $PKG_MANAGER in
    brew)
      brew tap oven-sh/bun 2>/dev/null || true
      brew install bun
      ;;
    *)
      # Use official installer for Linux
      echo "  Installing bun via official installer..."
      curl -fsSL https://bun.sh/install | bash
      # Source the updated PATH
      export BUN_INSTALL="$HOME/.bun"
      export PATH="$BUN_INSTALL/bin:$PATH"
      ;;
  esac
}

echo -e "${BLUE}claude-turbo-search dependency installer${NC}"
echo ""
echo -e "OS: ${YELLOW}$OS${NC}, Package Manager: ${YELLOW}$PKG_MANAGER${NC}"
echo ""
echo "Checking dependencies..."
echo ""

MISSING=()

check_command "rg" "ripgrep" || MISSING+=("ripgrep")
check_command "fzf" "fzf" || MISSING+=("fzf")
check_command "jq" "jq" || MISSING+=("jq")
check_command "bun" "bun" || MISSING+=("bun")
check_command "qmd" "qmd" || MISSING+=("qmd")
check_command "python3" "python3" || MISSING+=("python3")
check_command "go" "go" || MISSING+=("go")
check_command "sqlite3" "sqlite3" || MISSING+=("sqlite3")

echo ""

if [ ${#MISSING[@]} -eq 0 ]; then
  echo -e "${GREEN}All dependencies installed!${NC}"
  exit 0
fi

if [ "$CHECK_ONLY" = true ]; then
  echo -e "${YELLOW}Missing: ${MISSING[*]}${NC}"
  exit 1
fi

if [ "$PKG_MANAGER" = "unknown" ]; then
  echo -e "${RED}Error: No supported package manager found.${NC}"
  echo ""
  echo "Supported package managers:"
  echo "  - macOS: Homebrew (brew)"
  echo "  - Linux: apt-get, dnf, pacman"
  echo ""
  echo "Please install a package manager or install dependencies manually."
  exit 1
fi

echo -e "${YELLOW}Missing dependencies: ${MISSING[*]}${NC}"
echo ""
read -p "Install missing dependencies? [Y/n] " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]] && [[ -n $REPLY ]]; then
  echo "Skipping installation."
  exit 1
fi

for dep in "${MISSING[@]}"; do
  echo ""
  echo -e "${YELLOW}Installing $dep...${NC}"

  case $dep in
    ripgrep)
      # brew: ripgrep, apt: ripgrep, dnf: ripgrep, pacman: ripgrep
      install_package "ripgrep" "ripgrep" "ripgrep" "ripgrep"
      ;;
    fzf)
      # brew: fzf, apt: fzf, dnf: fzf, pacman: fzf
      install_package "fzf" "fzf" "fzf" "fzf"
      ;;
    jq)
      # brew: jq, apt: jq, dnf: jq, pacman: jq
      install_package "jq" "jq" "jq" "jq"
      ;;
    python3)
      install_package "python" "python3" "python3" "python"
      ;;
    go)
      install_package "go" "golang-go" "golang" "go"
      ;;
    sqlite3)
      install_package "sqlite" "sqlite3" "sqlite" "sqlite"
      ;;
    bun)
      install_bun
      ;;
    qmd)
      # Ensure bun is in PATH (may have just been installed)
      if [ -d "$HOME/.bun/bin" ]; then
        export PATH="$HOME/.bun/bin:$PATH"
      fi

      if ! command -v bun &> /dev/null; then
        echo -e "${RED}Error: bun must be installed first for qmd${NC}"
        exit 1
      fi
      echo "  Installing qmd globally with bun..."
      bun install -g @tobilu/qmd@2.8.3
      echo -e "${YELLOW}  Note: QMD will download ~1.7GB of models on first use${NC}"
      ;;
  esac

  echo -e "${GREEN}  ✓ $dep installed${NC}"
done

echo ""
echo -e "${GREEN}All dependencies installed!${NC}"
