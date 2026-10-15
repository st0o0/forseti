## ADDED Requirements

### Requirement: DHCP lease count metric
The system SHALL expose `forseti_dhcp_leases_active{target}` as a Prometheus gauge representing the number of active DHCP leases on each Pi-hole target.

#### Scenario: DHCP active with leases
- **WHEN** target `pihole-1` has DHCP enabled with 12 active leases
- **THEN** `forseti_dhcp_leases_active{target="pihole-1"}` SHALL be 12

#### Scenario: DHCP active with no leases
- **WHEN** target `pihole-1` has DHCP enabled but no active leases
- **THEN** `forseti_dhcp_leases_active{target="pihole-1"}` SHALL be 0

### Requirement: DHCP API integration
The Pi-hole client SHALL provide a `GetDHCPLeases()` method that fetches active leases from `GET /api/dhcp/leases`.

#### Scenario: Fetch leases successfully
- **WHEN** the Pi-hole DHCP server is active
- **THEN** `GetDHCPLeases()` SHALL return the list of active leases

#### Scenario: DHCP disabled on Pi-hole
- **WHEN** the Pi-hole DHCP server is not active and the API returns an empty response or error
- **THEN** the collector SHALL log a warning and set the lease gauge to 0

### Requirement: DHCP metric controlled by collector toggle
The `forseti_dhcp_leases_active` metric SHALL only be registered and collected when the `dhcp` collector toggle is enabled.

#### Scenario: Toggle disabled
- **WHEN** `metrics.collectors.dhcp` is `false`
- **THEN** no `forseti_dhcp_leases_active` metric SHALL be registered and `GetDHCPLeases()` SHALL not be called

#### Scenario: Toggle enabled
- **WHEN** `metrics.collectors.dhcp` is `true`
- **THEN** `forseti_dhcp_leases_active` SHALL be registered and populated during each collection cycle
