package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type RDSInstanceInfo struct {
	ID            string
	Engine        string
	EngineVersion string
	Class         string
	Status        string
	Endpoint      string
	MultiAZ       bool
	StorageGB     int32
	ClusterID     string // non-empty if part of an Aurora cluster
}

type RDSClusterInfo struct {
	ID             string
	Engine         string
	EngineVersion  string
	Status         string
	Endpoint       string
	ReaderEndpoint string
	Members        int
	MultiAZ        bool
}

func (c *Client) ListRDSInstances(ctx context.Context) ([]RDSInstanceInfo, error) {
	var instances []RDSInstanceInfo
	p := rds.NewDescribeDBInstancesPaginator(c.rds, &rds.DescribeDBInstancesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing RDS instances: %w", err)
		}
		for _, db := range page.DBInstances {
			endpoint := ""
			if db.Endpoint != nil {
				port := int32(0)
				if db.Endpoint.Port != nil {
					port = *db.Endpoint.Port
				}
				endpoint = fmt.Sprintf("%s:%d", awssdk.ToString(db.Endpoint.Address), port)
			}
			clusterID := ""
			if db.DBClusterIdentifier != nil {
				clusterID = awssdk.ToString(db.DBClusterIdentifier)
			}
			var storageGB int32
			if db.AllocatedStorage != nil {
				storageGB = *db.AllocatedStorage
			}
			instances = append(instances, RDSInstanceInfo{
				ID:            awssdk.ToString(db.DBInstanceIdentifier),
				Engine:        awssdk.ToString(db.Engine),
				EngineVersion: awssdk.ToString(db.EngineVersion),
				Class:         awssdk.ToString(db.DBInstanceClass),
				Status:        awssdk.ToString(db.DBInstanceStatus),
				Endpoint:      endpoint,
				MultiAZ:       db.MultiAZ != nil && *db.MultiAZ,
				StorageGB:     storageGB,
				ClusterID:     clusterID,
			})
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].ID < instances[j].ID
	})
	return instances, nil
}

func (c *Client) ListRDSClusters(ctx context.Context) ([]RDSClusterInfo, error) {
	var clusters []RDSClusterInfo
	p := rds.NewDescribeDBClustersPaginator(c.rds, &rds.DescribeDBClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing RDS clusters: %w", err)
		}
		for _, cl := range page.DBClusters {
			clusters = append(clusters, RDSClusterInfo{
				ID:             awssdk.ToString(cl.DBClusterIdentifier),
				Engine:         awssdk.ToString(cl.Engine),
				EngineVersion:  awssdk.ToString(cl.EngineVersion),
				Status:         awssdk.ToString(cl.Status),
				Endpoint:       awssdk.ToString(cl.Endpoint),
				ReaderEndpoint: awssdk.ToString(cl.ReaderEndpoint),
				Members:        len(cl.DBClusterMembers),
				MultiAZ:        cl.MultiAZ != nil && *cl.MultiAZ,
			})
		}
	}
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})
	return clusters, nil
}
