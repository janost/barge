# ECS Task Restart & Force Deploy Actions

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add task restart (stop task) and force new deployment actions to the TUI dashboard.

**Architecture:** Two new AWS client methods (`StopTask`, `ForceNewDeployment`) and two new actions wired into existing resources. Task restart is an action on `ECSTaskResource`. Force deploy is an action on `ECSServiceResource`. Both trigger API calls and re-fetch data on completion.

**Tech Stack:** Go, AWS ECS SDK, Bubble Tea

---

### Task 1: Add StopTask and ForceNewDeployment AWS client methods

**Files:**
- Modify: `aws/ecs.go`

**Step 1: Add StopTask method**

```go
func (c *Client) StopTask(ctx context.Context, cluster, taskID, reason string) error {
	_, err := c.ecs.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(cluster),
		Task:    aws.String(taskID),
		Reason:  aws.String(reason),
	})
	if err != nil {
		return fmt.Errorf("stopping task: %w", err)
	}
	return nil
}
```

**Step 2: Add ForceNewDeployment method**

```go
func (c *Client) ForceNewDeployment(ctx context.Context, cluster, service string) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(cluster),
		Service:            aws.String(service),
		ForceNewDeployment: true,
	})
	if err != nil {
		return fmt.Errorf("forcing new deployment: %w", err)
	}
	return nil
}
```

**Step 3: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add aws/ecs.go
git commit -m "feat: add StopTask and ForceNewDeployment AWS client methods"
```

---

### Task 2: Add Restart action to ECSTaskResource

**Files:**
- Modify: `tui/dashboard/ecs_task.go`
- Modify: `tui/dashboard/actions.go`

**Step 1: Add a generic API action factory to actions.go**

The existing actions are all subprocess-based (BorderedExec). We need a new pattern for actions that call AWS APIs and return to the table. Add to `actions.go`:

```go
// apiResultMsg is sent when an API action completes.
type apiResultMsg struct {
	message string
	err     error
}

// NewAPIAction creates an action that calls an AWS API and returns a result message.
func NewAPIAction(name string, fn func() error, successMsg string) Action {
	return Action{
		Name: name,
		Run: func() tea.Cmd {
			return func() tea.Msg {
				err := fn()
				if err != nil {
					return apiResultMsg{"", err}
				}
				return apiResultMsg{successMsg, nil}
			}
		},
	}
}
```

**Step 2: Handle apiResultMsg in dashboard model**

In `tui/dashboard/model.go`, add a case in `Update()` to handle `apiResultMsg`:

```go
case apiResultMsg:
    m.state = stateTable
    if msg.err != nil {
        m.message = fmt.Sprintf("Error: %v", msg.err)
    } else {
        m.message = msg.message
    }
    return m, m.currentResource().FetchCmd(m.client)
```

**Step 3: Add restart action to ECSTaskResource**

Modify `Actions()` in `ecs_task.go` to include a "Restart Task" option. The action needs access to the AWS client, so we need to store it on the resource:

Add `client *bargeaws.Client` field to `ECSTaskResource`.

Modify `FetchCmd` to store the client reference:

```go
func (r *ECSTaskResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	r.client = client
	return func() tea.Msg {
		tasks, err := client.ListTasks(context.Background(), r.cluster, r.service)
		return ecsTaskFetchMsg{tasks, err}
	}
}
```

Modify `Actions()`:

```go
func (r *ECSTaskResource) Actions(row table.Row) []Action {
	taskID := row[0]
	for _, t := range r.tasks {
		if t.ID == taskID {
			container, err := bargeaws.ResolveContainer(t.Containers, "")
			if err != nil {
				return nil
			}
			title := fmt.Sprintf("ECS: %s/%s (%s)", r.service, t.ID, container.Name)
			actions := []Action{
				ECSExecAction(r.cluster, t.ID, container.Name, title),
			}
			if r.client != nil {
				actions = append(actions, NewAPIAction(
					"Restart Task",
					func() error {
						return r.client.StopTask(context.Background(), r.cluster, t.ID, "Restarted via barge")
					},
					fmt.Sprintf("Task %s restart initiated", t.ID),
				))
			}
			return actions
		}
	}
	return nil
}
```

**Step 4: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/dashboard/actions.go tui/dashboard/model.go tui/dashboard/ecs_task.go
git commit -m "feat: add task restart action and API action infrastructure"
```

---

### Task 3: Add Force New Deployment action to ECSServiceResource

**Files:**
- Modify: `tui/dashboard/ecs_service.go`

**Step 1: Store client reference and add Actions**

Add `client *bargeaws.Client` to `ECSServiceResource`.

Modify `FetchCmd`:
```go
func (r *ECSServiceResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	r.client = client
	return func() tea.Msg {
		services, err := client.ListServices(context.Background(), r.cluster)
		return ecsServiceFetchMsg{services, err}
	}
}
```

Replace `Actions`:
```go
func (r *ECSServiceResource) Actions(row table.Row) []Action {
	if r.client == nil {
		return nil
	}
	serviceName := row[0]
	return []Action{
		NewAPIAction(
			"Force New Deployment",
			func() error {
				return r.client.ForceNewDeployment(context.Background(), r.cluster, serviceName)
			},
			fmt.Sprintf("New deployment triggered for %s", serviceName),
		),
	}
}
```

**Important:** Since ECSServiceResource is Drillable (Enter drills to tasks), Actions() is only shown if we change the behavior. Currently, Enter always drills. We need to change the model so that Drillable resources can ALSO have actions accessible via a different key.

**Alternative:** Don't use Actions. Instead, make force-deploy a keybind in the model (e.g., `d` for deploy). But that's a model change.

**Simplest approach:** Since ECSServiceResource is Drillable, the user never sees Actions. Instead, make the force-deploy available from the status view (ECSStatusResource) as an action. The status view is not Drillable, so Actions will show.

Modify `ECSStatusResource` instead:

```go
func (r *ECSStatusResource) FetchCmd(client *bargeaws.Client) tea.Cmd {
	r.client = client
	// ... existing fetch logic
}

func (r *ECSStatusResource) Actions(row table.Row) []Action {
	if r.client == nil {
		return nil
	}
	return []Action{
		NewAPIAction(
			"Force New Deployment",
			func() error {
				return r.client.ForceNewDeployment(context.Background(), r.cluster, r.service)
			},
			fmt.Sprintf("New deployment triggered for %s", r.service),
		),
	}
}
```

Add `client *bargeaws.Client` field to `ECSStatusResource`.

**Step 2: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 3: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add tui/dashboard/ecs_status.go
git commit -m "feat: add force new deployment action to ECS Status view"
```
