package gee

import (
	"html/template"
	"log"
	"net/http"
	"path"
	"strings"
)

type HandlerFunc func(*Context)

type (
	RouterGroup struct {
		prefix string
		middlewares []HandlerFunc
		parent *RouterGroup
		engine *Engine
	}

	Engine struct {
		*RouterGroup
		router *router
		groups []*RouterGroup
		//将所有的模板加载进内存
		htmlTemplates *template.Template // for html render
		//有的自定义模板渲染函数
		funcMap template.FuncMap // for html render
	}

)

func New() *Engine {
	engine := &Engine{router: newRouter()}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

// Default use Logger() & Recovery middlewares
func Default() *Engine {
	engine := New()
	engine.Use(Logger(), Recovery())
	return engine
}

func (group *RouterGroup) Group(prefix string) *RouterGroup{
	engine := group.engine
	newGroup := &RouterGroup{
		prefix: group.prefix + prefix,
		parent: group,
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup)
	return newGroup
}

// Use is defined to add middleware to the group
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
	group.middlewares = append(group.middlewares, middlewares...)
}

func (group *RouterGroup) addRoute(method string,comp string,handler HandlerFunc) {
	pattern := group.prefix + comp
	log.Printf("Route %4s -%s",method,pattern)
	group.engine.router.addRoute(method, pattern, handler)
}
/*
func (engine *Engine) GET(pattern string,handler HandlerFunc) {
	engine.addRoute("GET",pattern,handler)
}*/

func (group *RouterGroup) GET(pattern string,handler HandlerFunc) {
	group.addRoute("GET",pattern,handler)
}

func (group *RouterGroup) POST(pattern string,handler HandlerFunc) {
	group.addRoute("POST",pattern,handler)
}

func (group *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	//绝对路径
	absolutePath := path.Join(group.prefix, relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
	return func(c *Context) {
		file := c.Param("filepath")
		// Check if file exists and/or if we have permission to access it
		if _, err := fs.Open(file); err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		fileServer.ServeHTTP(c.Writer, c.Req)
	}
}

//相对路径
func (group *RouterGroup) Static(relativePath string, root string) {
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	urlPattern := path.Join(relativePath, "/*filepath")
	// Register GET handlers
	group.GET(urlPattern, handler)
}
func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
}

func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr,engine)
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter,req *http.Request) {
	var middlewares []HandlerFunc
	for _,group := range engine.groups {
		if strings.HasPrefix(req.URL.Path,group.prefix) {
			middlewares = append(middlewares,group.middlewares...)
		}
	}
	c := newContext(w, req)
	c.handlers = middlewares
	c.engine = engine
	engine.router.handle(c)
}