# Security Policy

## Supported versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Report security issues privately by opening a [GitHub Security Advisory](https://github.com/Jubblin/omni-kubeconfig/security/advisories/new) or emailing the repository owner.

Include:

- Description of the issue
- Steps to reproduce
- Impact assessment
- Suggested fix (if any)

We aim to acknowledge reports within 5 business days.

## Scope

This tool handles kubeconfig files and Omni API credentials on the local machine. Treat merged kubeconfig output and `~/.talos/keys` as sensitive. Do not commit omniconfig, PGP keys, or kubeconfig files to this repository.
