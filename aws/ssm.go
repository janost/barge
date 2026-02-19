package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type InstanceInfo struct {
	ID           string
	Name         string
	Platform     string
	IPAddress    string
	ComputerName string
}

func (c *Client) ListInstances(ctx context.Context) ([]InstanceInfo, error) {
	var instances []InstanceInfo

	paginator := ssm.NewDescribeInstanceInformationPaginator(c.ssm, &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    awssdk.String("PingStatus"),
				Values: []string{"Online"},
			},
		},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing SSM instances: %w", err)
		}
		for _, inst := range page.InstanceInformationList {
			// Only include EC2 instances (skip managed/hybrid nodes)
			id := awssdk.ToString(inst.InstanceId)
			if inst.ResourceType != ssmtypes.ResourceTypeEc2Instance {
				continue
			}

			instances = append(instances, InstanceInfo{
				ID:           id,
				Platform:     string(inst.PlatformType),
				IPAddress:    awssdk.ToString(inst.IPAddress),
				ComputerName: awssdk.ToString(inst.ComputerName),
			})
		}
	}

	// Enrich with EC2 Name tags (best-effort)
	instances = c.enrichInstanceNames(ctx, instances)

	sort.Slice(instances, func(i, j int) bool {
		// Named instances first, then by name, then by ID
		if (instances[i].Name != "") != (instances[j].Name != "") {
			return instances[i].Name != ""
		}
		if instances[i].Name != instances[j].Name {
			return instances[i].Name < instances[j].Name
		}
		return instances[i].ID < instances[j].ID
	})

	return instances, nil
}

func (c *Client) enrichInstanceNames(ctx context.Context, instances []InstanceInfo) []InstanceInfo {
	if len(instances) == 0 {
		return instances
	}

	ids := make([]string, len(instances))
	for i, inst := range instances {
		ids[i] = inst.ID
	}

	// EC2 DescribeInstances accepts up to 1000 instance IDs
	out, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		// Best-effort: if we lack permissions, just skip name enrichment
		return instances
	}

	nameMap := make(map[string]string)
	for _, res := range out.Reservations {
		for _, inst := range res.Instances {
			id := awssdk.ToString(inst.InstanceId)
			for _, tag := range inst.Tags {
				if awssdk.ToString(tag.Key) == "Name" {
					nameMap[id] = awssdk.ToString(tag.Value)
					break
				}
			}
		}
	}

	// Handle pagination for large instance lists
	for out.NextToken != nil {
		out, err = c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			InstanceIds: ids,
			NextToken:   out.NextToken,
		})
		if err != nil {
			break
		}
		for _, res := range out.Reservations {
			for _, inst := range res.Instances {
				id := awssdk.ToString(inst.InstanceId)
				for _, tag := range inst.Tags {
					if awssdk.ToString(tag.Key) == "Name" {
						nameMap[id] = awssdk.ToString(tag.Value)
						break
					}
				}
			}
		}
	}

	for i := range instances {
		if name, ok := nameMap[instances[i].ID]; ok {
			instances[i].Name = name
		}
	}

	return instances
}
