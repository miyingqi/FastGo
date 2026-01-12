package FastGo

type Router struct {
	route map[string]*RouteNode
}

// NewRouter 创建路由
func NewRouter() *Router {
	return &Router{
		route: make(map[string]*RouteNode),
	}
}

// 获取路由
func (r *Router) getRoute(method string) *RouteNode {
	route, ok := r.route[method]
	if !ok {
		route = route.NewTire()
	}
	r.route[method] = route
	return route
}

// 添加路由
func (r *Router) addRoute(path, method string, handlers HandleFuncChain) {
	route := r.getRoute(method)
	route.Insert(path, handlers)
}

// HandleHTTP  请求处理
func (r *Router) HandleHTTP(c *Context) {
	method := c.method
	path := c.path

	// 获取对应方法的路由树
	routeNode, ok := r.route[method]
	if !ok {
		HTTPNotFound(c)
		return
	}

	// 在路由树中查找匹配的节点
	matchedNode, _ := routeNode.FindChild(path)
	if matchedNode == nil || matchedNode.handlers == nil {
		HTTPNotFound(c)
		return
	}

	// 执行处理器链
	for _, handler := range matchedNode.handlers {
		handler(c)
	}
}
