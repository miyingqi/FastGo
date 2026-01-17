package FastGo

import (
	"github.com/miyingqi/FastGo/internal/route"
)

type GroupRouter struct {
	prefix string
	router *Router
}

type route_ struct {
	route_  *route.RouteNode
	handles map[string]HandleFuncChain
}

func (r *route_) setHandles(hash string, handles ...HandlerFunc) {
	child, ok := r.handles[hash]
	if !ok {
		child = handles
		r.handles[hash] = child
	}
}

type Router struct {
	route map[string]*route_
}

// NewRouter 创建路由
func NewRouter() *Router {
	return &Router{
		route: make(map[string]*route_),
	}
}

// 获取路由
func (r *Router) getRoute(method string) *route_ {
	node, ok := r.route[method]
	if !ok {
		// 修正：NewTire是routeNode的方法，需用空节点调用（原代码逻辑正确，但写法可优化）
		// 创建新的 route_ 对象并初始化所有字段
		node = &route_{
			route_:  (&route.RouteNode{}).NewTire(),   // 初始化路由树
			handles: make(map[string]HandleFuncChain), // 初始化 handles map
		}
	}
	r.route[method] = node
	return node
}

// 添加路由
func (r *Router) addRoute(path, method string, handlers ...HandlerFunc) {
	node := r.getRoute(method)
	hash := node.route_.Insert(path)
	node.setHandles(hash, handlers...)

}

// HandleHTTP  请求处理
func (r *Router) HandleHTTP(c *Context) {
	method := c.GetMethod()
	path := c.GetPath()

	// 获取对应方法的路由树
	routeNode, ok := r.route[method]
	if !ok {
		HTTPNotFound(c)
		return
	}

	// 在路由树中查找匹配的节点
	matchedNode, params := routeNode.route_.FindChild(path)
	if matchedNode == nil || matchedNode.GetHashs() == "" {
		HTTPNotFound(c)
		return
	}
	// 绑定参数到Context
	for key, value := range params {
		c.SetParam(key, value)
	}
	// 执行处理器链
	for _, handler := range routeNode.handles[matchedNode.GetHashs()] {
		handler(c)
	}
}
