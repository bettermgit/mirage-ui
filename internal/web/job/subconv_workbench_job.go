package job

import (
	"github.com/mhsanaei/3x-ui/v3/internal/sub"
)

// SubConvWorkbenchJob periodically fetches the engine workbench upstream
// subscription sources so publication links serve fresh nodes.
type SubConvWorkbenchJob struct {
	subService *sub.SubService
}

func NewSubConvWorkbenchJob() *SubConvWorkbenchJob {
	return &SubConvWorkbenchJob{subService: sub.NewSubService("")}
}

// Run is invoked by the cron scheduler.
func (j *SubConvWorkbenchJob) Run() {
	j.subService.RefreshWorkbenchSources()
}
