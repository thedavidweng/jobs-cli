# Installed binary is jobs-cli

The shipped executable is named `jobs-cli`, not `jobs`. On bash/zsh, `jobs` is a shell builtin for job control, so a PATH binary named `jobs` would not run when users type `jobs` in an interactive shell. Users who want a short alias can define one themselves. The Go module and repo remain `jobs-cli`; command docs refer to `jobs-cli <command>`.
