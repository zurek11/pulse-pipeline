# Security — Public Repository

This is a PUBLIC GitHub repository. Every file committed is visible to everyone.

## Absolute Rules

- NEVER include API keys, tokens, passwords, or secrets in any file
- NEVER hardcode connection strings with credentials
- NEVER commit `.env` files — use `.env.example` with placeholder values only
- NEVER include personal data (emails, phone numbers, addresses) in code or comments
- All configuration goes through environment variables

## If You Need a Secret

1. Add it to `.env` (which is gitignored)
2. Add a placeholder to `.env.example`
3. Reference it in code via `os.Getenv("VARIABLE_NAME")`
4. Document the variable in CLAUDE.md

## Docker Compose

- Credentials in docker-compose.yml are LOCAL DEV ONLY (admin/admin, no auth)
- These are not secrets — they only work on localhost
- Never use production credentials in docker-compose.yml
