package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

type CFNStackInfo struct {
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	Outputs   []CFNOutput
}

type CFNOutput struct {
	Key         string
	Value       string
	Description string
}

type CFNStackEvent struct {
	Timestamp    time.Time
	ResourceType string
	LogicalID    string
	Status       string
	Reason       string
}

func (c *Client) ListCFNStacks(ctx context.Context) ([]CFNStackInfo, error) {
	var stacks []CFNStackInfo
	p := cloudformation.NewListStacksPaginator(c.cfn, &cloudformation.ListStacksInput{
		StackStatusFilter: []cfntypes.StackStatus{
			cfntypes.StackStatusCreateComplete,
			cfntypes.StackStatusUpdateComplete,
			cfntypes.StackStatusUpdateRollbackComplete,
			cfntypes.StackStatusRollbackComplete,
			cfntypes.StackStatusCreateInProgress,
			cfntypes.StackStatusUpdateInProgress,
			cfntypes.StackStatusDeleteInProgress,
		},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing stacks: %w", err)
		}
		for _, s := range page.StackSummaries {
			var updatedAt time.Time
			if s.LastUpdatedTime != nil {
				updatedAt = *s.LastUpdatedTime
			}
			var createdAt time.Time
			if s.CreationTime != nil {
				createdAt = *s.CreationTime
			}
			stacks = append(stacks, CFNStackInfo{
				Name:      awssdk.ToString(s.StackName),
				Status:    string(s.StackStatus),
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			})
		}
	}
	sort.Slice(stacks, func(i, j int) bool {
		return stacks[i].Name < stacks[j].Name
	})
	return stacks, nil
}

func (c *Client) ListCFNStackEvents(ctx context.Context, stackName string) ([]CFNStackEvent, error) {
	out, err := c.cfn.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
		StackName: awssdk.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describing stack events: %w", err)
	}

	events := make([]CFNStackEvent, 0, len(out.StackEvents))
	for _, e := range out.StackEvents {
		var ts time.Time
		if e.Timestamp != nil {
			ts = *e.Timestamp
		}
		events = append(events, CFNStackEvent{
			Timestamp:    ts,
			ResourceType: awssdk.ToString(e.ResourceType),
			LogicalID:    awssdk.ToString(e.LogicalResourceId),
			Status:       string(e.ResourceStatus),
			Reason:       awssdk.ToString(e.ResourceStatusReason),
		})
	}
	return events, nil
}
