package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// IsConflict is the predicate the optimistic-lock retry loop hinges on: a
// version-mismatch PutCourt surfaces as ConditionalCheckFailedException and must
// be recognised (even when wrapped), while ordinary errors must NOT trigger a retry.
func TestIsConflict(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"generic", errors.New("boom"), false},
		{"conditional-check-failed", &types.ConditionalCheckFailedException{}, true},
		{"wrapped-conditional", fmt.Errorf("put court: %w", &types.ConditionalCheckFailedException{}), true},
		{"other-aws-error", &types.ProvisionedThroughputExceededException{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsConflict(c.err); got != c.want {
				t.Errorf("IsConflict(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
