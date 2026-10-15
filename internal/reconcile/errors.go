package reconcile

import (
	"fmt"

	"github.com/st0o0/forseti/internal/redact"
)

// ItemError is a failed write of one item (or one batch of Count items) of a
// resource type during Apply. Resource uses the change-map names (groups,
// adlists, deny, allow, local_dns, cname, clients); Action is add, update or
// delete.
type ItemError struct {
	Resource string
	Action   string
	Key      string
	Count    int
	Err      error
}

func (e *ItemError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("%s %s (%d items): %v", e.Action, e.Resource, e.Count, e.Err)
	}
	return fmt.Sprintf("%s %s %q: %v", e.Action, e.Resource, e.Key, e.Err)
}

func (e *ItemError) Unwrap() error { return e.Err }

func itemErr(resource, action, key string, err error) *ItemError {
	if resource == "adlists" {
		key = redact.URL(key)
	}
	return &ItemError{Resource: resource, Action: action, Key: key, Count: 1, Err: err}
}

func batchErr(resource string, n int, err error) *ItemError {
	return &ItemError{Resource: resource, Action: "delete", Count: n, Err: err}
}
