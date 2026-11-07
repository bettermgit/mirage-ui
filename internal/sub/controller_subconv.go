package sub

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// subConvPath is the fixed public path for engine workbench publications.
// The random token in the URL is the only credential.
const subConvPath = "subconv"

// subConvPublication renders a private publication link created by the
// engine workbench (merged panel clients + upstream sources).
func (a *SUBController) subConvPublication(c *gin.Context) {
	token := c.Param("token")
	pub, err := a.subService.FindWorkbenchPublication(token)
	if err != nil {
		writeSubError(c, err)
		return
	}
	if pub == nil || !pub.Enable {
		writeSubError(c, nil)
		return
	}
	_, host, _, _ := a.subService.ResolveRequest(c)
	out, err := a.subClashService.RenderPublication(pub, host)
	if err != nil {
		logger.Warningf("sub: workbench publication %q render failed: %v", pub.Name, err)
		c.String(http.StatusBadGateway, "%s", err.Error())
		return
	}
	c.Writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(pub.Name)))
	c.Data(http.StatusOK, "application/yaml; charset=utf-8", []byte(out))
}
