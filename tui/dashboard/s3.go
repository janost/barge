package dashboard

import (
	"context"
	"fmt"

	bridgeaws "github.com/janost/bridge/aws"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// --- S3 Buckets ---

type s3BucketFetchMsg struct {
	buckets []bridgeaws.S3BucketInfo
	err     error
}

type S3BucketResource struct {
	buckets []bridgeaws.S3BucketInfo
	err     error
}

func NewS3BucketResource() *S3BucketResource {
	return &S3BucketResource{}
}

func (r *S3BucketResource) Name() string { return "S3 Buckets" }

func (r *S3BucketResource) Columns() []Column {
	return []Column{
		{"BUCKET", 40},
		{"CREATED", 20},
	}
}

func (r *S3BucketResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
	return func() tea.Msg {
		buckets, err := client.ListS3Buckets(context.Background())
		return s3BucketFetchMsg{buckets, err}
	}
}

func (r *S3BucketResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(s3BucketFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.buckets = m.buckets
		}
		return true
	}
	return false
}

func (r *S3BucketResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.buckets))
	for i, b := range r.buckets {
		created := ""
		if !b.CreatedAt.IsZero() {
			created = b.CreatedAt.Format("2006-01-02")
		}
		rows[i] = table.Row{b.Name, created}
	}
	return rows
}

func (r *S3BucketResource) Actions(row table.Row) []Action { return nil }
func (r *S3BucketResource) Error() error                   { return r.err }

func (r *S3BucketResource) ChildResource(row table.Row) (string, Resource) {
	return row[0], NewS3ObjectResource(row[0], "")
}

// --- S3 Objects ---

type s3ObjectFetchMsg struct {
	objects []bridgeaws.S3ObjectInfo
	err     error
}

type S3ObjectResource struct {
	bucket  string
	prefix  string
	objects []bridgeaws.S3ObjectInfo
	err     error
}

func NewS3ObjectResource(bucket, prefix string) *S3ObjectResource {
	return &S3ObjectResource{bucket: bucket, prefix: prefix}
}

func (r *S3ObjectResource) Name() string { return "Objects" }

func (r *S3ObjectResource) Columns() []Column {
	return []Column{
		{"NAME", 40},
		{"SIZE", 12},
		{"LAST MODIFIED", 20},
	}
}

func (r *S3ObjectResource) FetchCmd(client *bridgeaws.Client) tea.Cmd {
	return func() tea.Msg {
		objects, err := client.ListS3Objects(context.Background(), r.bucket, r.prefix)
		return s3ObjectFetchMsg{objects, err}
	}
}

func (r *S3ObjectResource) HandleMsg(msg tea.Msg) bool {
	if m, ok := msg.(s3ObjectFetchMsg); ok {
		r.err = m.err
		if m.err == nil {
			r.objects = m.objects
		}
		return true
	}
	return false
}

func (r *S3ObjectResource) Rows() []table.Row {
	rows := make([]table.Row, len(r.objects))
	for i, obj := range r.objects {
		size := ""
		lastMod := ""
		if obj.IsPrefix {
			size = "-"
		} else {
			size = formatSize(obj.Size)
			if !obj.LastModified.IsZero() {
				lastMod = obj.LastModified.Format("2006-01-02 15:04:05")
			}
		}
		rows[i] = table.Row{obj.DisplayName, size, lastMod}
	}
	return rows
}

func (r *S3ObjectResource) Actions(row table.Row) []Action { return nil }
func (r *S3ObjectResource) Error() error                   { return r.err }

// ChildResource drills into prefixes (directories). For non-prefix items,
// returns a new S3ObjectResource with the same prefix (effectively a no-op refresh).
func (r *S3ObjectResource) ChildResource(row table.Row) (string, Resource) {
	for _, obj := range r.objects {
		if obj.DisplayName == row[0] && obj.IsPrefix {
			return row[0], NewS3ObjectResource(r.bucket, obj.Key)
		}
	}
	// Non-prefix: drill into same view (no-op, just refreshes)
	return row[0], NewS3ObjectResource(r.bucket, r.prefix)
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
