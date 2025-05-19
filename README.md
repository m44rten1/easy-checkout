# Easy Checkout

A simple Go program that enhances your git branch checkout experience by providing an interactive fuzzy finder interface with branches sorted by their last checkout time.

## Features

- Interactive fuzzy search for git branches
- Branches are ordered by last checkout time (using git reflog)
- Preview window showing branch name and last checkout timestamp
- Similar to using `git branch | fzf | xargs git checkout` but with better branch ordering

## Installation

### Via Homebrew (Recommended)

```bash
# Add the tap
brew tap m44rten1/easy-checkout

# Install easy-checkout
brew install easy-checkout
```

### Manual Installation

1. Make sure you have Go installed on your system
2. Clone this repository
3. Run:
   ```bash
   go install
   ```

## Usage

Simply run `easy-checkout` in any git repository:

```bash
easy-checkout
```

### Optional: Add an alias

Add this to your `.zshrc` or `.bashrc`:

```bash
alias check='easy-checkout'
```

## Key Bindings

- Type to filter branches
- `↑`/`↓` or `Ctrl-p`/`Ctrl-n` to move selection
- `Enter` to select and checkout
- `Esc` or `Ctrl-c` to cancel

## Requirements

- Git 2.22.0 or later
- Go 1.21 or later