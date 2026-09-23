#!/bin/bash
# Post-install script for gc package

set -e

# Update shell completions if available
if command -v update-shell-completions &> /dev/null; then
    update-shell-completions
fi

# Inform user about setup
echo "GitCode CLI (gc/gitcode) has been installed successfully!"
echo "Run 'gitcode doctor install' to inspect PATH conflicts with pip, npm, Homebrew, or archive installs."
echo ""
echo "To get started:"
echo "  1. Run 'gitcode auth login' or 'gc auth login' to authenticate"
echo "  2. Set your token: export GC_TOKEN=your_token"
echo ""
echo "Project README: https://gitcode.com/atomgit-cli/cli"
echo "Command docs: https://gitcode.com/atomgit-cli/cli/blob/main/docs/COMMANDS.md"

exit 0
