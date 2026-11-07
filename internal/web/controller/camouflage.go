package controller

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

// CamouflageController manages the built-in cover-site server and its
// automatic fallback wiring on a master inbound.
type CamouflageController struct {
	camouflageService service.CamouflageService
	xrayService       service.XrayService
}

func NewCamouflageController(g *gin.RouterGroup) *CamouflageController {
	a := &CamouflageController{}
	a.initRouter(g)
	return a
}

func (a *CamouflageController) initRouter(g *gin.RouterGroup) {
	g.GET("/status", a.getStatus)
	g.POST("/apply", a.apply)
	g.POST("/disable", a.disable)
}

func (a *CamouflageController) getStatus(c *gin.Context) {
	status, err := a.camouflageService.GetStatus()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, status, nil)
}

func (a *CamouflageController) apply(c *gin.Context) {
	req, ok := middleware.BindAndValidate[service.CamouflageApplyRequest](c)
	if !ok {
		return
	}
	status, err := a.camouflageService.Apply(*req)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), status, nil)
	a.xrayService.SetToNeedRestart()
}

func (a *CamouflageController) disable(c *gin.Context) {
	if err := a.camouflageService.Disable(); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), nil)
	a.xrayService.SetToNeedRestart()
}
