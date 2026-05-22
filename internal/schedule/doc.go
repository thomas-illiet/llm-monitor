// Package schedule groups the task registry and monitor task packages.
//
// The executable wires subpackages under this module:
//   - runner contains generic task registration primitives.
//   - queue contains Asynq task constructors, workers, and periodic scheduling.
//   - tasks contains monitor-specific business tasks.
package schedule
