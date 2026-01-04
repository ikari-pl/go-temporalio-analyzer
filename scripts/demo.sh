#!/bin/bash
# Demo script for temporal-analyzer
# This script showcases the main features of the tool
# Run this while terminalizer is recording

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Typing simulation
type_cmd() {
    echo -ne "${BLUE}➜${NC} "
    for ((i=0; i<${#1}; i++)); do
        echo -n "${1:$i:1}"
        sleep 0.03
    done
    echo
    sleep 0.3
    eval "$1"
}

clear
echo -e "${GREEN}⚡ temporal-analyzer demo${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
sleep 1

# Show version
type_cmd "temporal-analyzer --version"
sleep 1
echo ""

# Show help
echo -e "${YELLOW}# Let's see the available options:${NC}"
sleep 0.5
type_cmd "temporal-analyzer --help"
sleep 2
echo ""

# Run lint mode
echo -e "${YELLOW}# Run in lint mode for CI/CD:${NC}"
sleep 0.5
type_cmd "temporal-analyzer --lint ."
sleep 2
echo ""

# List lint rules
echo -e "${YELLOW}# List available lint rules:${NC}"
sleep 0.5
type_cmd "temporal-analyzer --lint-rules | head -40"
sleep 2
echo ""

# Export to different formats
echo -e "${YELLOW}# Export to Mermaid diagram:${NC}"
sleep 0.5
type_cmd "temporal-analyzer --format mermaid . | head -20"
sleep 2
echo ""

# Launch TUI
echo -e "${YELLOW}# Launch interactive TUI:${NC}"
sleep 0.5
echo -e "${BLUE}➜${NC} temporal-analyzer ."
echo "(Press 'q' to quit, '?' for help)"
sleep 1

# The actual TUI launch
temporal-analyzer .

