package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type LogConfig struct {
	LogGroup     string
	StreamPrefix string
	Region       string
}

func (c *Client) ResolveLogConfig(ctx context.Context, cluster, service string) (LogConfig, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  awssdk.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return LogConfig{}, fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return LogConfig{}, fmt.Errorf("service %q not found in cluster %q", service, cluster)
	}

	taskDefArn := awssdk.ToString(out.Services[0].TaskDefinition)

	tdOut, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: awssdk.String(taskDefArn),
	})
	if err != nil {
		return LogConfig{}, fmt.Errorf("describing task definition: %w", err)
	}

	for _, cd := range tdOut.TaskDefinition.ContainerDefinitions {
		if cd.LogConfiguration == nil {
			continue
		}
		if cd.LogConfiguration.LogDriver != ecstypes.LogDriverAwslogs {
			continue
		}
		opts := cd.LogConfiguration.Options
		logGroup := opts["awslogs-group"]
		streamPrefix := opts["awslogs-stream-prefix"]
		region := opts["awslogs-region"]

		if logGroup == "" {
			continue
		}

		return LogConfig{
			LogGroup:     logGroup,
			StreamPrefix: streamPrefix,
			Region:       region,
		}, nil
	}

	return LogConfig{}, fmt.Errorf("no container with awslogs driver found in task definition %s", shortName(taskDefArn))
}

type LogEvent struct {
	Timestamp time.Time
	Message   string
	Stream    string
}

func (c *Client) FetchLogs(ctx context.Context, logGroup, streamPrefix, filterPattern string, startTime time.Time, nextToken *string) ([]LogEvent, *string, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: awssdk.String(logGroup),
		StartTime:    awssdk.Int64(startTime.UnixMilli()),
		Interleaved:  awssdk.Bool(true),
	}

	if streamPrefix != "" {
		input.LogStreamNamePrefix = awssdk.String(streamPrefix)
	}
	if filterPattern != "" {
		input.FilterPattern = awssdk.String(filterPattern)
	}
	if nextToken != nil {
		input.NextToken = nextToken
	}

	out, err := c.cwl.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("filtering log events: %w", err)
	}

	events := make([]LogEvent, len(out.Events))
	for i, e := range out.Events {
		ts := time.UnixMilli(awssdk.ToInt64(e.Timestamp))
		events[i] = LogEvent{
			Timestamp: ts,
			Message:   awssdk.ToString(e.Message),
			Stream:    awssdk.ToString(e.LogStreamName),
		}
	}

	return events, out.NextToken, nil
}
