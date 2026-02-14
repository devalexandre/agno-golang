# Code Style Guide

## Naming Conventions
- **Variables**: Use camelCase for local variables
- **Functions**: Use PascalCase for exported, camelCase for unexported
- **Constants**: Use UPPER_SNAKE_CASE for constants
- **Packages**: Use lowercase, single-word names

## Error Handling
- Always check returned errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Use sentinel errors for expected error conditions

## Function Guidelines
- Functions should do one thing
- Keep functions under 50 lines
- Limit function parameters to 4; use structs for more

## Security
- Never hardcode credentials
- Validate all user input
- Use parameterized queries for database access
