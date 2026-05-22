package queue

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"

	"llmservicemonitor/internal/config"
	"llmservicemonitor/internal/schedule/tasks/shared"
)

// Client enqueues monitor tasks and inspects their queue state.
type Client struct {
	cfg       config.Config
	client    *asynq.Client
	inspector *asynq.Inspector
}

// EnqueuedTask describes one task accepted by Redis.
type EnqueuedTask struct {
	ID      string `json:"id"`
	Queue   string `json:"queue"`
	Type    string `json:"type"`
	ModelID string `json:"model_id,omitempty"`
	State   string `json:"state"`
}

// JobStatus describes the latest known state of one queued task.
type JobStatus struct {
	ID          string     `json:"id"`
	Queue       string     `json:"queue"`
	Type        string     `json:"type,omitempty"`
	ModelID     string     `json:"model_id,omitempty"`
	State       string     `json:"state"`
	Error       string     `json:"error,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// NewClient opens the Asynq client and inspector used by the API and scheduler.
func NewClient(cfg config.Config) (*Client, error) {
	redisOpt := RedisOpt(cfg)
	client := asynq.NewClient(redisOpt)
	if err := client.Ping(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Client{
		cfg:       cfg,
		client:    client,
		inspector: asynq.NewInspector(redisOpt),
	}, nil
}

// Close releases Redis resources.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.client != nil {
		err = errors.Join(err, c.client.Close())
	}
	if c.inspector != nil {
		err = errors.Join(err, c.inspector.Close())
	}
	return err
}

func (c *Client) enqueue(ctx context.Context, task *asynq.Task, modelID string) (EnqueuedTask, error) {
	info, err := c.client.EnqueueContext(ctx, task, manualTaskOptions(c.cfg)...)
	if err != nil {
		return EnqueuedTask{}, err
	}
	return EnqueuedTask{
		ID:      info.ID,
		Queue:   info.Queue,
		Type:    info.Type,
		ModelID: modelID,
		State:   info.State.String(),
	}, nil
}

// InspectJobs returns queue state for task IDs in the configured queue.
func (c *Client) InspectJobs(_ context.Context, ids []string) ([]JobStatus, error) {
	statuses := make([]JobStatus, 0, len(ids))
	now := time.Now().UTC()
	for _, id := range ids {
		info, err := c.inspector.GetTaskInfo(c.cfg.Asynq.Queue, id)
		if err != nil {
			if errors.Is(err, asynq.ErrTaskNotFound) {
				statuses = append(statuses, JobStatus{ID: id, Queue: c.cfg.Asynq.Queue, State: "not_found"})
				continue
			}
			return nil, err
		}
		status := jobStatusFromTaskInfo(info)
		if expiredPendingJob(info, now) {
			if err := c.inspector.DeleteTask(info.Queue, info.ID); err != nil {
				return nil, err
			}
			status.State = "expired"
			status.Error = "manual check expired before a worker started it"
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func jobStatusFromTaskInfo(info *asynq.TaskInfo) JobStatus {
	status := JobStatus{
		ID:    info.ID,
		Queue: info.Queue,
		Type:  info.Type,
		State: info.State.String(),
		Error: info.LastErr,
	}
	if !info.CompletedAt.IsZero() {
		completedAt := info.CompletedAt
		status.CompletedAt = &completedAt
	}
	if info.Type == shared.ModelRunTaskName {
		if payload, err := shared.UnmarshalModelRunPayload(info.Payload); err == nil {
			status.ModelID = payload.ModelID
		}
	}
	return status
}

func expiredPendingJob(info *asynq.TaskInfo, now time.Time) bool {
	return info.State == asynq.TaskStatePending && !info.Deadline.IsZero() && !now.Before(info.Deadline)
}

// ScheduledModelRuns returns the next scheduler enqueue time by runnable model.
func (c *Client) ScheduledModelRuns(ctx context.Context) (map[string]time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := c.inspector.SchedulerEntries()
	if err != nil {
		return nil, err
	}
	return modelRunNextChecks(entries), nil
}

func modelRunNextChecks(entries []*asynq.SchedulerEntry) map[string]time.Time {
	nextChecks := map[string]time.Time{}
	for _, entry := range entries {
		if entry == nil || entry.Task == nil || entry.Task.Type() != shared.ModelRunTaskName || entry.Next.IsZero() {
			continue
		}
		payload, err := shared.UnmarshalModelRunPayload(entry.Task.Payload())
		if err != nil {
			continue
		}
		next := entry.Next.Add(processInDelay(entry.Opts)).UTC()
		current, exists := nextChecks[payload.ModelID]
		if !exists || next.Before(current) {
			nextChecks[payload.ModelID] = next
		}
	}
	return nextChecks
}

func processInDelay(options []asynq.Option) time.Duration {
	var delay time.Duration
	for _, option := range options {
		if option == nil || option.Type() != asynq.ProcessInOpt {
			continue
		}
		value, ok := option.Value().(time.Duration)
		if ok {
			delay = value
		}
	}
	return delay
}
