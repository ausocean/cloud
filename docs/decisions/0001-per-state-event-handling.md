# 0001. Per-State Event Handling

> **Author:** David Sutton `<davidsutton@ausocean.org>` \
> **Date:** <Badge type="info" text="2026-06-04" />

## Context

OceanTV uses state machines (broadcast and hardware state machines) to manage the lifecycle of a broadcast. Event management was previously handled centrally via a `handleEvent` function that dispatched to typed handler methods for each event type. Each event specific handler than used a type switch on the current state of the state machine to determine transitions.

This approach had several drawbacks:

- **Readability** Due to the increasing number of events and states as the broadcast manager matured the event handling became a very large file, with many event handler functions all in one place. This made it difficult to trace behaviour of a state, as its transitions were spread over multiple event handler methods.
- **Scalability concerns**: As new states and events were added, the central switch statements grew linearly, increasing the risk of accidental omission or incorrect handling.

## Decision

We will refactor event handling from a state-machine-centralised pattern to a per-state pattern, guided by the following design:

1. **Introduce a `stateWithEventHandler` interface** that states can optionally implement:

   ```go
   type stateWithEventHandler interface {
       handleEvent(sm *broadcastStateMachine, event event)
   }
   ```

2. **Modify `broadcastStateMachine.handleEvent`** to check whether the current state implements this interface. If it does, the event is delegated to the state; otherwise, the old centralised handling is used as a fallback.

3. **Migrate states incrementally**: Each state is converted one at a time by adding a `handleEvent` method that encapsulates all events that state can receive and the corresponding transitions. This allows the refactor to proceed as a series of small, reviewable changes.

4. **Remove migrated event branches** from the centralised handler methods once all states are converted.

## Consequences

### Positive

- **Localised reasoning**: All event handling for a state is in one place, making it easy to enumerate every incoming event and outgoing transition.
- **Incremental adoption**: The interface+fallback mechanism allows piecemeal conversion without a big-bang rewrite.
- **Clearer diffs**: State changes are confined to the state's own file, reducing noise in the state machine file.
- **Easier onboarding**: New developers can understand a state's behaviour by reading a single file.

### Negative

- **Duplicated patterns**: Shared event handling logic (e.g., timeout behaviour across "starting" states) may be duplicated rather than centralised.
- **Increased boilerplate**: Each state requires a `handleEvent` method even if it only handles one or two events.
- **State coupling**: States now reference other states directly in their `handleEvent` methods (to call `sm.transition`), whereas the centralised handler kept transition knowledge in the state machine.

## Future Work

- Convert the remaining broadcast states to the per-state pattern.
- Consider whether `stateWithEventHandler` should be promoted to the base `state` interface once all states have been converted.
- Evaluate whether a `stateMachineCtxer` interface would allow this pattern to be shared with the hardware state machine.
