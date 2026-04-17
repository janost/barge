package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

type ASGInfo struct {
	Name        string
	MinSize     int32
	MaxSize     int32
	Desired     int32
	InService   int32
	InstanceIDs []string
}

func (c *Client) ListASGs(ctx context.Context) ([]ASGInfo, error) {
	var asgs []ASGInfo
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(c.asg, &autoscaling.DescribeAutoScalingGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing ASGs: %w", err)
		}
		for _, g := range page.AutoScalingGroups {
			var ids []string
			var inService int32
			for _, inst := range g.Instances {
				ids = append(ids, awssdk.ToString(inst.InstanceId))
				if inst.LifecycleState == asgtypes.LifecycleStateInService {
					inService++
				}
			}
			asgs = append(asgs, ASGInfo{
				Name:        awssdk.ToString(g.AutoScalingGroupName),
				MinSize:     awssdk.ToInt32(g.MinSize),
				MaxSize:     awssdk.ToInt32(g.MaxSize),
				Desired:     awssdk.ToInt32(g.DesiredCapacity),
				InService:   inService,
				InstanceIDs: ids,
			})
		}
	}
	sort.Slice(asgs, func(i, j int) bool {
		return asgs[i].Name < asgs[j].Name
	})
	return asgs, nil
}
