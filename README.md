# cit - Commit Interactive Terminal

**🚧 This is a work in progress 🚧**

*Note: Most of this code was generated using GitHub Copilot Agent and Claude 3.7 Sonnet.*

## Overview

`cit` is a terminal-based interactive Git commit browser and checkout tool built with Go. It provides a convenient TUI (Text User Interface) for navigating through your Git repository's commit history and switching between commits and branches.

## Features

- Full-screen terminal UI powered by the `tview` library
- Browse and scroll through all commits in your Git repository
- View commit details including hash, date, author, and message
- Highlight the current HEAD position
- Display uncommitted changes
- Interactive branch selection when multiple branches point to the same commit
- Checkout commits with proper handling of both branch switching and detached HEAD states
- Automatic branch information caching for improved performance
- Real-time UI updates when Git state changes

## Usage

1. Navigate to a Git repository in your terminal
2. Run the `cit` command
3. Use arrow keys to navigate through commits
4. Press Enter on a commit to checkout
   - For commits with multiple branches, select the desired branch first
   - Confirm checkout with y/n
5. Press Escape to exit

### Key Bindings

- ↑/↓: Navigate commits
- Page Up/Down: Scroll page by page
- Enter: Select/checkout commit
- ←/→: Navigate between branch options (when multiple branches available)
- y/n: Confirm/cancel checkout
- Esc: Exit selection mode or exit application

## Requirements

- Go 1.18 or later
- Git installed and accessible in PATH
- A terminal that supports TUI applications

## Installation

```bash
# Clone the repository
git clone https://github.com/soramimi/cit.git

# Build the project
cd cit
go build

# Run the application
./cit
```

## Dependencies

- [github.com/gdamore/tcell/v2](https://github.com/gdamore/tcell) - Terminal cell library
- [github.com/rivo/tview](https://github.com/rivo/tview) - Terminal UI library

