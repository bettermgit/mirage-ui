package controller

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/sub"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

// SubConvSource is one upstream subscription URL aggregated by the engine
// workbench.
type SubConvSource struct {
	ID          string `json:"id" example:"a1b2c3d4"`
	Name        string `json:"name" example:"Airport A"`
	URL         string `json:"url" example:"https://example.com/sub"`
	Enable      bool   `json:"enable" example:"true"`
	LastFetchAt int64  `json:"lastFetchAt" example:"1730000000"`
	LastCount   int    `json:"lastCount" example:"12"`
	LastError   string `json:"lastError" example:""`
}

// SubConvPublication is a generated private subscription link.
type SubConvPublication struct {
	ID        string   `json:"id" example:"p1q2r3s4"`
	Token     string   `json:"token" example:"Zx8Kp3Vn2Mq7Wt5Y"`
	Name      string   `json:"name" example:"Main line"`
	SourceIDs []string `json:"sourceIds" example:"[\"a1b2c3d4\"]"`
	SubIDs    []string `json:"subIds" example:"[\"kvu2buy7zi4r9yaa\"]"`
	Enable    bool     `json:"enable" example:"true"`
	CreatedAt int64    `json:"createdAt" example:"1730000000"`
}

// SubConvState is the full workbench snapshot for the settings page.
type SubConvState struct {
	Sources      []SubConvSource      `json:"sources"`
	Publications []SubConvPublication `json:"publications"`
}

// SubConvIDRequest targets one source or publication.
type SubConvIDRequest struct {
	ID string `json:"id" example:"a1b2c3d4"`
}

// SubConvPreviewRequest renders a draft merge without creating a publication.
type SubConvPreviewRequest struct {
	SourceIDs []string `json:"sourceIds" example:"[\"a1b2c3d4\"]"`
	SubIDs    []string `json:"subIds" example:"[\"kvu2buy7zi4r9yaa\"]"`
}

// SubConvPreview carries the rendered mihomo document.
type SubConvPreview struct {
	YAML      string `json:"yaml"`
	NodeCount int    `json:"nodeCount" example:"12"`
}

// SubConvController exposes the mihomo engine workbench (upstream sources and
// private publication links) to the panel.
type SubConvController struct {
	subService     *sub.SubService
	subClashSvc    *sub.SubClashService
	settingService service.SettingService
}

func NewSubConvController(g *gin.RouterGroup) *SubConvController {
	a := &SubConvController{}
	a.settingService = service.SettingService{}
	template, err := a.settingService.GetRemarkTemplate()
	if err != nil {
		template = ""
	}
	a.subService = sub.NewSubService(template)
	a.subClashSvc = sub.NewSubClashService(a.subService)
	a.initRouter(g)
	return a
}

func (a *SubConvController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/subconv")
	g.GET("/state", a.getState)
	g.POST("/source/save", a.saveSource)
	g.POST("/source/delete", a.deleteSource)
	g.POST("/source/refresh", a.refreshSource)
	g.POST("/publication/save", a.savePublication)
	g.POST("/publication/delete", a.deletePublication)
	g.POST("/preview", a.preview)
}

func toSubSource(in SubConvSource) sub.WorkbenchSource {
	return sub.WorkbenchSource{ID: in.ID, Name: in.Name, URL: in.URL, Enable: in.Enable}
}

func fromSubSource(in sub.WorkbenchSource) SubConvSource {
	return SubConvSource{ID: in.ID, Name: in.Name, URL: in.URL, Enable: in.Enable, LastFetchAt: in.LastFetchAt, LastCount: in.LastCount, LastError: in.LastError}
}

func fromSubPublication(in sub.WorkbenchPublication) SubConvPublication {
	return SubConvPublication{ID: in.ID, Token: in.Token, Name: in.Name, SourceIDs: in.SourceIDs, SubIDs: in.SubIDs, Enable: in.Enable, CreatedAt: in.CreatedAt}
}

func (a *SubConvController) state() (SubConvState, error) {
	state := SubConvState{Sources: []SubConvSource{}, Publications: []SubConvPublication{}}
	sources, err := a.subService.GetWorkbenchSources()
	if err != nil {
		return state, err
	}
	publications, err := a.subService.GetWorkbenchPublications()
	if err != nil {
		return state, err
	}
	for _, s := range sources {
		state.Sources = append(state.Sources, fromSubSource(s))
	}
	for _, p := range publications {
		state.Publications = append(state.Publications, fromSubPublication(p))
	}
	return state, nil
}

func (a *SubConvController) getState(c *gin.Context) {
	state, err := a.state()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, state, nil)
}

func (a *SubConvController) saveSource(c *gin.Context) {
	req, ok := middleware.BindAndValidate[SubConvSource](c)
	if !ok {
		return
	}
	saved, err := a.subService.SaveWorkbenchSource(toSubSource(*req))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	// Fresh source rows save before their first fetch so failures stay visible.
	if saved.URL != "" {
		if _, err := a.subService.FetchWorkbenchSource(saved.ID); err != nil {
			saved.LastError = err.Error()
		}
	}
	out := fromSubSource(saved)
	state, _ := a.state()
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), gin.H{"source": out, "state": state}, nil)
}

func (a *SubConvController) deleteSource(c *gin.Context) {
	req, ok := middleware.BindAndValidate[SubConvIDRequest](c)
	if !ok {
		return
	}
	if err := a.subService.DeleteWorkbenchSource(req.ID); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), nil)
}

func (a *SubConvController) refreshSource(c *gin.Context) {
	req, ok := middleware.BindAndValidate[SubConvIDRequest](c)
	if !ok {
		return
	}
	nodes, err := a.subService.FetchWorkbenchSource(req.ID)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	state, _ := a.state()
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), gin.H{"count": len(nodes), "state": state}, nil)
}

func (a *SubConvController) savePublication(c *gin.Context) {
	type publicationRequest struct {
		Publication SubConvPublication `json:"publication"`
		RotateToken bool               `json:"rotateToken"`
	}
	req, ok := middleware.BindAndValidate[publicationRequest](c)
	if !ok {
		return
	}
	pub := sub.WorkbenchPublication{
		ID: req.Publication.ID, Name: req.Publication.Name,
		SourceIDs: req.Publication.SourceIDs, SubIDs: req.Publication.SubIDs,
		Enable: req.Publication.Enable,
	}
	saved, err := a.subService.SaveWorkbenchPublication(pub, req.RotateToken)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), fromSubPublication(saved), nil)
}

func (a *SubConvController) deletePublication(c *gin.Context) {
	req, ok := middleware.BindAndValidate[SubConvIDRequest](c)
	if !ok {
		return
	}
	if err := a.subService.DeleteWorkbenchPublication(req.ID); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), nil)
}

func (a *SubConvController) preview(c *gin.Context) {
	req, ok := middleware.BindAndValidate[SubConvPreviewRequest](c)
	if !ok {
		return
	}
	pub := &sub.WorkbenchPublication{SourceIDs: req.SourceIDs, SubIDs: req.SubIDs, Enable: true}
	yamlDoc, err := a.subClashSvc.RenderPublication(pub, c.Request.Host)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, SubConvPreview{YAML: yamlDoc, NodeCount: countWorkbenchProxies(yamlDoc)}, nil)
}

// countWorkbenchProxies counts items of the top-level proxies: block.
func countWorkbenchProxies(doc string) int {
	count, inProxies := 0, false
	for _, line := range strings.Split(doc, "\n") {
		if line != "" && line[0] != ' ' && line[0] != '#' && line[0] != '-' {
			inProxies = strings.HasPrefix(line, "proxies:")
			continue
		}
		if inProxies && strings.HasPrefix(line, "  - ") {
			count++
		}
	}
	return count
}
