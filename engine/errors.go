package engine

import (
	"fmt"

	"github.com/quonaro/lota/config"
)

// GroupError is returned by Run when the resolved path points to a group
// rather than a command. The caller can use errors.As to detect it and
// print group help or take any other action.
type GroupError struct {
	Path   string
	Groups []*config.Group
}

func (e *GroupError) Error() string {
	return fmt.Sprintf("%q is a group, not a command", e.Path)
}
