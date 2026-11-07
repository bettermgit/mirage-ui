package controller

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	"github.com/gin-gonic/gin"
)

// getQuickTemplates lists the one-click inbound presets.
func (a *InboundController) getQuickTemplates(c *gin.Context) {
	jsonObj(c, a.inboundService.QuickTemplates(), nil)
}

// quickAddInbound creates a fully randomized inbound from a preset, applying
// it through the same AddInbound path as manual creation.
func (a *InboundController) quickAddInbound(c *gin.Context) {
	opts, ok := middleware.BindAndValidate[service.QuickAddOptions](c)
	if !ok {
		return
	}
	user := session.GetLoginUser(c)
	inbound, needRestart, err := a.inboundServiceFor(c).QuickAdd(user.Id, *opts)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundCreateSuccess"), inbound, nil)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	a.broadcastInboundsUpdate(user.Id)
	notifyClientsChanged()
}
