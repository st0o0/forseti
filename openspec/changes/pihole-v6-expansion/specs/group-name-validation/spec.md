## ADDED Requirements

### Requirement: Group name sanitization
The system SHALL validate group names at config parse time and reject names containing carriage return (CR), line feed (LF), or null bytes. This prevents CRLF injection via the Pi-hole API.

#### Scenario: Valid group name
- **WHEN** a group name contains only printable characters without CR, LF, or null
- **THEN** config validation SHALL accept the group name

#### Scenario: Group name with CRLF
- **WHEN** a group name contains CR (\r) or LF (\n) characters
- **THEN** config validation SHALL fail with an error identifying the invalid group name

#### Scenario: Group name with null byte
- **WHEN** a group name contains a null byte (\x00)
- **THEN** config validation SHALL fail with an error identifying the invalid group name

### Requirement: Sync mode group name validation
The system SHALL validate group names received from the primary Pi-hole during sync. If the primary contains a group name with invalid characters, the system SHALL log a warning and skip syncing that group.

#### Scenario: Primary has invalid group name
- **WHEN** the primary Pi-hole returns a group name containing CR, LF, or null
- **THEN** the sync SHALL log a warning, skip the invalid group, and continue syncing other groups
