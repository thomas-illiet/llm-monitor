package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"llmservicemonitor/internal/schedule/queue"
)

// runChecks enqueues manual health, inventory, or model probe work from the dashboard.
func (r *Router) runChecks(w http.ResponseWriter, req *http.Request) {
	if r.taskQueue == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "task queue is not configured"})
		return
	}
	var payload RunChecksRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	payload.Scope = strings.TrimSpace(strings.ToLower(payload.Scope))
	payload.ModelID = strings.TrimSpace(payload.ModelID)
	var jobs []queue.EnqueuedTask
	var err error
	switch payload.Scope {
	case "all":
		jobs, err = r.enqueueAllChecks(req)
	case "model":
		if payload.ModelID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_id is required for model scope"})
			return
		}
		jobs, err = r.enqueueModelCheck(req, payload.ModelID)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be all or model"})
		return
	}
	if err != nil {
		switch err.(type) {
		case errModelNotRunnable:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		case errRunnableModelsUnavailable:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, RunChecksResponse{Jobs: jobs})
}

func (r *Router) enqueueAllChecks(req *http.Request) ([]queue.EnqueuedTask, error) {
	ctx := req.Context()
	jobs := make([]queue.EnqueuedTask, 0)
	httpJob, err := r.taskQueue.EnqueueHTTPCheck(ctx)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, httpJob)
	authJob, err := r.taskQueue.EnqueueAuthCheck(ctx)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, authJob)
	snapshotJob, err := r.taskQueue.EnqueueModelSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, snapshotJob)
	modelStore, ok := r.store.(runnableModelStore)
	if !ok {
		return jobs, nil
	}
	models, err := modelStore.RunnableModels(ctx)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		job, err := r.taskQueue.EnqueueModelRun(ctx, model, "manual")
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (r *Router) enqueueModelCheck(req *http.Request, modelID string) ([]queue.EnqueuedTask, error) {
	modelStore, ok := r.store.(runnableModelStore)
	if !ok {
		return nil, errRunnableModelsUnavailable{}
	}
	models, err := modelStore.RunnableModels(req.Context())
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if model.ModelID == modelID {
			job, err := r.taskQueue.EnqueueModelRun(req.Context(), model, "manual")
			if err != nil {
				return nil, err
			}
			return []queue.EnqueuedTask{job}, nil
		}
	}
	return nil, errModelNotRunnable{modelID: modelID}
}

// checkJobs returns manual queue status used by dashboard spinners.
func (r *Router) checkJobs(w http.ResponseWriter, req *http.Request) {
	if r.taskQueue == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "task queue is not configured"})
		return
	}
	ids := parseJobIDs(req.URL.Query()["ids"])
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids query parameter is required"})
		return
	}
	statuses, err := r.taskQueue.InspectJobs(req.Context(), ids)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, CheckJobsResponse{Jobs: statuses})
}

type errRunnableModelsUnavailable struct{}

func (errRunnableModelsUnavailable) Error() string {
	return "runnable model store is not available"
}

type errModelNotRunnable struct {
	modelID string
}

func (e errModelNotRunnable) Error() string {
	return "model " + e.modelID + " is not runnable"
}

func parseJobIDs(values []string) []string {
	seen := map[string]struct{}{}
	var ids []string
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			id := strings.TrimSpace(part)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	return ids
}
