package search

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

type Client struct {
	cli       meilisearch.ServiceManager
	IndexName string
	BatchSize int
}

func NewClient(host, apiKey, indexName string, batchSize int) *Client {
	return &Client{
		cli:       meilisearch.New(host, meilisearch.WithAPIKey(apiKey)),
		IndexName: indexName,
		BatchSize: batchSize,
	}
}

func (c *Client) Health(ctx context.Context) error {
	_ = ctx
	_, err := c.cli.ServiceReader().Health()
	return err
}

func (c *Client) Index() meilisearch.IndexManager {
	return c.cli.Index(c.IndexName)
}

func (c *Client) EnsureIndex(ctx context.Context) error {
	if _, err := c.cli.Index(c.IndexName).FetchInfoWithContext(ctx); err == nil {
		return nil
	}
	_, err := c.cli.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{Uid: c.IndexName})
	return err
}

func (c *Client) WaitTask(ctx context.Context, taskUID int64) error {
	t := time.NewTicker(300 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			task, err := c.cli.GetTask(taskUID)
			if err != nil {
				return err
			}
			if task.Status == "succeeded" {
				return nil
			}
			if task.Status == "failed" {
				return fmt.Errorf("meilisearch task failed: %d", taskUID)
			}
		}
	}
}
