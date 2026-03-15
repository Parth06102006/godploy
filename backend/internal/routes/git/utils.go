package gitroutes

import (
	"context"
	"fmt"

	"github.com/Roshan-anand/godploy/internal/db"
)

// remove the session data
func removeSession(query *db.Queries, state string) {
	if err := query.DeleteRedirectSession(context.Background(), state); err != nil {
		fmt.Println("Error deleting redirect session:", err)
	}
}
