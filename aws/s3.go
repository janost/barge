package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3BucketInfo struct {
	Name      string
	CreatedAt time.Time
}

type S3ObjectInfo struct {
	Key          string
	DisplayName  string
	Size         int64
	LastModified time.Time
	IsPrefix     bool // true for "directory" prefixes
}

func (c *Client) ListS3Buckets(ctx context.Context) ([]S3BucketInfo, error) {
	out, err := c.s3c.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}

	buckets := make([]S3BucketInfo, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		var createdAt time.Time
		if b.CreationDate != nil {
			createdAt = *b.CreationDate
		}
		buckets = append(buckets, S3BucketInfo{
			Name:      awssdk.ToString(b.Name),
			CreatedAt: createdAt,
		})
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})
	return buckets, nil
}

func (c *Client) ListS3Objects(ctx context.Context, bucket, prefix string) ([]S3ObjectInfo, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    awssdk.String(bucket),
		Delimiter: awssdk.String("/"),
	}
	if prefix != "" {
		input.Prefix = awssdk.String(prefix)
	}

	var objects []S3ObjectInfo

	p := s3.NewListObjectsV2Paginator(c.s3c, input)
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing objects: %w", err)
		}

		// Add "directory" prefixes
		for _, cp := range page.CommonPrefixes {
			name := awssdk.ToString(cp.Prefix)
			display := strings.TrimPrefix(name, prefix)
			objects = append(objects, S3ObjectInfo{
				Key:         name,
				DisplayName: display,
				IsPrefix:    true,
			})
		}

		// Add objects
		for _, obj := range page.Contents {
			key := awssdk.ToString(obj.Key)
			if key == prefix {
				continue // skip the prefix itself
			}
			display := strings.TrimPrefix(key, prefix)
			var lastMod time.Time
			if obj.LastModified != nil {
				lastMod = *obj.LastModified
			}
			objects = append(objects, S3ObjectInfo{
				Key:          key,
				DisplayName:  display,
				Size:         awssdk.ToInt64(obj.Size),
				LastModified: lastMod,
				IsPrefix:     false,
			})
		}
	}

	return objects, nil
}
