# Log Redaction

## Purpose

Keeps credentials, tokens and raw upstream responses out of Forseti's logs and error messages, while preserving enough detail to diagnose problems.

## Requirements

### Requirement: Passwords are never logged
Target passwords and session IDs SHALL NOT appear in any log line or error message, including when a target configuration value is logged as a whole.

#### Scenario: Target logged as attribute
- **WHEN** a target configuration is passed to the logger as an attribute
- **THEN** the output SHALL contain the target name and host but not its password

### Requirement: URL redaction
URLs that appear in logs or errors (adlist addresses, target URLs) SHALL have userinfo removed and every query parameter value replaced by `REDACTED`. Scheme, host and path SHALL be preserved.

#### Scenario: Private blocklist token
- **WHEN** creating adlist `https://lists.example.org/x.txt?token=abc123` fails
- **THEN** the logged error SHALL contain `https://lists.example.org/x.txt?token=REDACTED` and SHALL NOT contain `abc123`

### Requirement: Bounded API error messages
Pi-hole API errors SHALL be rendered from the parsed error key and message when the response is JSON. Otherwise the raw body SHALL be truncated to at most 200 bytes with newlines collapsed.

#### Scenario: HTML error page from a proxy
- **WHEN** a reverse proxy returns a 502 HTML page of several kilobytes
- **THEN** the resulting error message SHALL contain the status code and at most 200 bytes of the body

### Requirement: Sensitive settings are not logged
Settings that carry secrets (passwords, password hashes, TOTP secrets, tokens, keys) SHALL be logged and printed as `[REDACTED]` when applied or diffed. Any mapped setting whose Pi-hole path matches such a name SHALL be marked sensitive.

#### Scenario: Guard against unmarked secrets
- **WHEN** a setting mapping with a path containing `password`, `pwhash`, `secret`, `totp`, `token` or `key` is added without being marked sensitive
- **THEN** the test suite SHALL fail
