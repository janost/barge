# Integration: Merge all new resources into picker

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Final integration step — merge all new resource registrations into picker.go after parallel worktrees are complete.

**Architecture:** Each parallel stream adds its own resource files but only the integration step modifies picker.go to avoid merge conflicts. This plan runs LAST after all other streams are merged.

**Tech Stack:** Go, git

**Prerequisites:** All 7 feature branches must be merged first.

---

### Task 1: Verify all resource files exist

**Step 1: Check that all expected files are present**

Expected files:
- `tui/dashboard/ecs_events.go`
- `tui/dashboard/ecs_status.go`
- `tui/dashboard/ecs_logs.go`
- `tui/dashboard/rds.go`
- `tui/dashboard/cfn.go`
- `tui/dashboard/s3.go`

Run: `ls tui/dashboard/ecs_events.go tui/dashboard/ecs_status.go tui/dashboard/ecs_logs.go tui/dashboard/rds.go tui/dashboard/cfn.go tui/dashboard/s3.go`

**Step 2: Verify all compile**

Run: `GOCACHE=/tmp go build ./...`

---

### Task 2: Update picker.go with all new entries

**Files:**
- Modify: `tui/dashboard/picker.go`

**Step 1: Update Rows()**

The final picker should have these entries:

```go
func (r *ResourcePicker) Rows() []table.Row {
	return []table.Row{
		{"EC2 Instances", "Managed instances with SSM agent"},
		{"Auto Scaling Groups", "EC2 Auto Scaling Groups"},
		{"ECS Clusters", "ECS clusters, services, and tasks"},
		{"ECS Task Definitions", "Task definition families"},
		{"ECS Service Status", "Deployments, tasks, and recent stops"},
		{"ECS Service Logs", "Recent CloudWatch logs for a service"},
		{"RDS Clusters", "Aurora clusters and member instances"},
		{"RDS Instances", "All RDS database instances"},
		{"CloudFormation Stacks", "Stack status and events"},
		{"S3 Buckets", "Browse S3 buckets and objects"},
	}
}
```

**Step 2: Update ChildResource()**

```go
func (r *ResourcePicker) ChildResource(row table.Row) (string, Resource) {
	switch row[0] {
	case "Auto Scaling Groups":
		return "Auto Scaling Groups", NewASGResource()
	case "ECS Clusters":
		return "ECS Clusters", NewECSClusterResource()
	case "ECS Task Definitions":
		return "ECS Task Definitions", NewECSTaskDefResource()
	case "ECS Service Status":
		return "ECS Service Status", NewECSClusterResourceForStatus()
	case "ECS Service Logs":
		return "ECS Service Logs", NewECSClusterResourceForLogs()
	case "RDS Clusters":
		return "RDS Clusters", NewRDSClusterResource()
	case "RDS Instances":
		return "RDS Instances", NewRDSInstanceResource("")
	case "CloudFormation Stacks":
		return "CloudFormation Stacks", NewCFNStackResource()
	case "S3 Buckets":
		return "S3 Buckets", NewS3BucketResource()
	default:
		return "EC2 Instances", NewEC2InstanceResource()
	}
}
```

**Step 3: Verify compilation**

Run: `GOCACHE=/tmp go build ./...`
Expected: PASS

**Step 4: Verify vet**

Run: `GOCACHE=/tmp go vet ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add tui/dashboard/picker.go
git commit -m "feat: register all new resources in picker"
```
