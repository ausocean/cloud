# Overview

Direct broadcasting is the most common usage of OceanTV at the time of writing. It is the simplest configuration of the OceanTV broadcast manager, where the camera sends video data directly to YouTube (or other streaming services in the future). This is primarily used for daily scheduled streams.

## Direct Broadcasting State Machine

Below is a State Diagram showing the transitions between states when broadcasting in direct mode.

![Direct Broadcasting State Machine](../images/oceantv/direct_sm.png)

## Events

Relevant event types (names are defined in `cmd/oceantv/broadcast_events.go`):

| Event | Purpose |
|---|---|
| `timeEvent{time.Time}` | Periodic clock tick used to advance the state machines and trigger scheduled checks. |
| `startEvent{}` / `finishEvent{}` | Schedule-based start and finish triggers (issued when current time is within configured start/end window). |
| `hardwareStartRequestEvent{}` / `hardwareStopRequestEvent{}` / `hardwareResetRequestEvent{}` | Requests sent to the hardware state machine to start, stop, or reset camera hardware. |
| `hardwareStartedEvent{}` / `hardwareStoppedEvent{}` | Hardware confirmations emitted when the camera begins or stops reporting (DeviceIsUp checks). |
| `startedEvent{}` / `startFailedEvent{err}` / `criticalFailureEvent{err}` | Results from broadcast start attempts: success, recoverable failure, or non‑recoverable critical failure. |
| `badHealthEvent{}` / `goodHealthEvent{}` | Health check outcomes; bad health can trigger fixes or state transitions. |
| `lowVoltageEvent{}` / `voltageRecoveredEvent{}` | Controller battery/voltage alarms used to prevent/delay starts and to trigger recovery behaviour. |
| `invalidConfigurationEvent{err}` | Configuration or sensor errors that usually disable or move the broadcast into a failure state. |
| `statusCheckDueEvent{}` / `chatMessageDueEvent{}` | Periodic maintenance triggers: status checks and scheduled chat messages. |

## Broadcast check crons

Each broadcast has a **Broadcast Check <UUID>** cron set to `@every 15s`.
OceanCron sends `/checkbroadcasts` a JSON payload containing that UUID, and the
endpoint loads only that broadcast at the site identified by its signed token.

Saving or creating a broadcast creates its cron. Changing `Enabled` updates the
cron's enabled state, including internal disables and secondary broadcasts.
Deleting through OceanBench calls OceanTV, which performs the disabled cleanup
and removes both the broadcast and its cron. Disabling through OceanBench also
performs the cleanup once, since disabled broadcasts no longer receive ticks.

Existing site-level **Broadcast Check** crons migrate automatically on their next
request: OceanTV creates jobs for all the site's broadcasts, preserving the old
job's target deployment, then removes the site-level job. Jobs for disabled
broadcasts remain disabled. The old job is retained if installing a replacement
fails, allowing a later request to retry. Legacy configurations without UUIDs
are assigned UUIDs during migration. Sites without an existing job can create
their individual jobs by saving their broadcasts.

OceanTV accepts `-cronurl` to select the OceanCron service (default:
`https://cron.cloudblue.org`). New jobs target the OceanTV host receiving the
save request; secondary jobs inherit the primary job's deployment.
