package FastGo

import "strings"

type nodeType uint8
type HandleFuncChain []HandlerFunc

const (
	static   nodeType = iota // 静态节点
	root                     // 根节点
	param                    // 参数节点，如 :id
	catchAll                 // 通配符节点，如 *path
)

type Router struct {
	route map[string]*routeNode
}

// RouteGroup 表示路由组
type RouteGroup struct {
	prefix   string
	handlers HandleFuncChain
	router   *Router
}

// NewRouter 创建路由
func NewRouter() *Router {
	return &Router{
		route: make(map[string]*routeNode),
	}
}

// Group 创建一个新的路由组
func (r *Router) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		prefix:   prefix,
		handlers: nil,
		router:   r,
	}
}

// Use 为路由组添加中间件
func (group *RouteGroup) Use(middleware ...HandlerFunc) {
	group.handlers = append(group.handlers, middleware...)
}

// GET 添加GET请求路由
func (group *RouteGroup) GET(path string, handler HandlerFunc) {
	group.addRoute("GET", path, handler)
}

// POST 添加POST请求路由
func (group *RouteGroup) POST(path string, handler HandlerFunc) {
	group.addRoute("POST", path, handler)
}

// PUT 添加PUT请求路由
func (group *RouteGroup) PUT(path string, handler HandlerFunc) {
	group.addRoute("PUT", path, handler)
}

// DELETE 添加DELETE请求路由
func (group *RouteGroup) DELETE(path string, handler HandlerFunc) {
	group.addRoute("DELETE", path, handler)
}

// PATCH 添加PATCH请求路由
func (group *RouteGroup) PATCH(path string, handler HandlerFunc) {
	group.addRoute("PATCH", path, handler)
}

// OPTIONS 添加OPTIONS请求路由
func (group *RouteGroup) OPTIONS(path string, handler HandlerFunc) {
	group.addRoute("OPTIONS", path, handler)
}

// HEAD 添加HEAD请求路由
func (group *RouteGroup) HEAD(path string, handler HandlerFunc) {
	group.addRoute("HEAD", path, handler)
}

// addRoute 为路由组添加路由
func (group *RouteGroup) addRoute(method, path string, handler HandlerFunc) {
	path = group.prefix + path
	handlers := make(HandleFuncChain, 0, len(group.handlers)+1)
	handlers = append(handlers, group.handlers...)
	handlers = append(handlers, handler)
	group.router.addRoute(path, method, handlers)
}

// getRoute 获取路由
func (r *Router) getRoute(method string) *routeNode {
	route, ok := r.route[method]
	if !ok {
		route = route.NewTire()
	}
	r.route[method] = route
	return route
}

// addRoute 添加路由
func (r *Router) addRoute(path, method string, handlers HandleFuncChain) {
	route := r.getRoute(method)
	route.Insert(path, handlers)
}

// HandleHTTP  请求处理
func (r *Router) HandleHTTP(c *Context) {
	method := c.method
	path := c.path

	routeNode, ok := r.route[method]
	if !ok {
		HTTPNotFound(c)
		return
	}

	matchedNode, params := routeNode.FindChild(path)
	if matchedNode == nil || matchedNode.Handlers == nil {
		HTTPNotFound(c)
		return
	}

	for key, value := range params {
		c.Params = append(c.Params, Param{Key: key, Value: value})
	}

	for _, handler := range matchedNode.Handlers {
		handler(c)
	}
}

// Router 上的路由方法
// GET 添加GET请求路由
func (r *Router) GET(path string, handler HandlerFunc) {
	r.addRoute(path, "GET", HandleFuncChain{handler})
}

// POST 添加POST请求路由
func (r *Router) POST(path string, handler HandlerFunc) {
	r.addRoute(path, "POST", HandleFuncChain{handler})
}

// PUT 添加PUT请求路由
func (r *Router) PUT(path string, handler HandlerFunc) {
	r.addRoute(path, "PUT", HandleFuncChain{handler})
}

// DELETE 添加DELETE请求路由
func (r *Router) DELETE(path string, handler HandlerFunc) {
	r.addRoute(path, "DELETE", HandleFuncChain{handler})
}

// PATCH 添加PATCH请求路由
func (r *Router) PATCH(path string, handler HandlerFunc) {
	r.addRoute(path, "PATCH", HandleFuncChain{handler})
}

// OPTIONS 添加OPTIONS请求路由
func (r *Router) OPTIONS(path string, handler HandlerFunc) {
	r.addRoute(path, "OPTIONS", HandleFuncChain{handler})
}

// HEAD 添加HEAD请求路由
func (r *Router) HEAD(path string, handler HandlerFunc) {
	r.addRoute(path, "HEAD", HandleFuncChain{handler})
}

// MergeRouter 合并另一个路由器的路由
func (r *Router) MergeRouter(other *Router) {
	for method, routeNode := range other.route {
		if existingRoute, exists := r.route[method]; exists {
			// 如果该HTTP方法已存在路由，则需要合并路由树
			mergeRouteNodes(existingRoute, routeNode)
		} else {
			// 如果该HTTP方法不存在路由，则直接复制整个路由树
			r.route[method] = routeNode
		}
	}
}

// mergeRouteNodes 合并两个路由节点树
func mergeRouteNodes(existing, other *routeNode) {
	// 这里需要实现具体的路由节点合并逻辑
	// 简单的合并策略：other节点覆盖existing节点
	// 但通常我们会想要递归合并而不是覆盖
	if other.Handlers != nil {
		existing.Handlers = other.Handlers
	}
	for _, otherChild := range other.children {
		// 使用 indices 优化查找过程
		var existingChild *routeNode
		if len(otherChild.path) > 0 {
			firstChar := otherChild.path[0:1]
			// 检查第一个字符是否在 indices 中
			if strings.Contains(existing.indices, firstChar) {
				// 只在对应首字符的节点中查找
				for _, ec := range existing.children {
					if ec.path[0:1] == firstChar && ec.path == otherChild.path {
						existingChild = ec
						break
					}
				}
			}
		} else {
			// 如果otherChild.path为空，则遍历所有子节点
			for _, ec := range existing.children {
				if ec.path == otherChild.path {
					existingChild = ec
					break
				}
			}
		}
		if existingChild != nil {
			// 如果找到匹配的子节点，递归合并
			mergeRouteNodes(existingChild, otherChild)
		} else {
			// 如果没有匹配的子节点，直接添加
			existing.children = append(existing.children, otherChild)
			existing.indices += otherChild.path[0:1]
		}
	}
}

// routeNode 路由节点
type routeNode struct {
	path      string          // 节点的路径（公共前缀）
	indices   string          // 子节点首字符索引
	children  []*routeNode    // 子节点
	Handlers  HandleFuncChain // 处理器链
	priority  uint32          // 节点优先级
	nType     nodeType        // 节点类型
	maxParams uint8           // 子树中最大参数数量
	wildChild bool            // 是否有通配符子节点
	paramName string
}

func (r *routeNode) Insert(path string, handlers HandleFuncChain) {
	if r == nil || len(path) == 0 {
		return
	}

	parts := splitPath(path)
	current := r

	for _, part := range parts {
		if part == "" {
			continue
		}

		var child *routeNode

		// 使用 indices 优化查找过程
		if len(part) > 0 {
			firstChar := part[0:1]
			// 检查第一个字符是否在 indices 中
			if strings.Contains(current.indices, firstChar) {
				// 只在对应首字符的节点中查找
				for _, c := range current.children {
					if c.path[0:1] == firstChar && c.path == part {
						child = c
						break
					}
				}
			}
		} else {
			// 如果part为空，则遍历所有子节点
			for _, c := range current.children {
				if c.path == part {
					child = c
					break
				}
			}
		}

		if child == nil {
			child = &routeNode{
				path:      part,
				indices:   "",
				children:  make([]*routeNode, 0),
				Handlers:  nil,
				priority:  current.priority + 1,
				nType:     static,
				maxParams: 0,
				wildChild: false,
				paramName: "",
			}

			if part[0] == ':' {
				child.nType = param
				child.paramName = strings.TrimPrefix(part, ":")
			} else if part[0] == '*' {
				child.nType = catchAll
				child.wildChild = true
			} else {
				child.nType = static
			}
			current.indices += part[0:1]
			current.children = append(current.children, child)
		}
		if current.maxParams < current.calculateMaxParams() {
			current.maxParams = current.calculateMaxParams()
		}

		current = child
	}

	current.Handlers = handlers
}

func (r *routeNode) NewTire() *routeNode {
	return &routeNode{
		path:      "",
		indices:   "",
		children:  make([]*routeNode, 0),
		priority:  0,
		maxParams: 0,
		nType:     root,
		wildChild: false,
		Handlers:  nil,
		paramName: "",
	}
}

// FindChild 获取路由
func (r *routeNode) FindChild(path string) (*routeNode, map[string]string) {
	if r == nil || path == "" {
		return nil, nil
	}

	parts := splitPath(path)
	current := r
	params := make(map[string]string)

	for _, part := range parts {
		if part == "" {
			continue
		}

		var child *routeNode
		found := false

		if len(current.children) == 0 {
			return nil, nil
		}

		// 首先尝试使用 indices 字段进行快速查找
		if len(part) > 0 {
			firstChar := part[0:1]
			// 检查第一个字符是否在 indices 中
			if strings.Contains(current.indices, firstChar) {
				// 在 children 中只查找对应首字符的节点
				for _, c := range current.children {
					if c.path[0:1] == firstChar {
						if c.path == part {
							child = c
							found = true
							break
						}
					}
				}
			}
		}

		// 如果没有找到静态匹配，尝试参数节点
		if !found {
			for _, c := range current.children {
				if c.nType == param {
					child = c
					found = true
					params[c.paramName] = part
					break
				}
			}
		}

		// 如果没有找到参数匹配，尝试通配符节点
		if !found {
			for _, c := range current.children {
				if c.nType == catchAll && c.wildChild {
					child = c
					found = true
					params[c.paramName] = part
					break
				}
			}
		}

		if !found {
			return nil, nil
		}

		current = child
	}

	return current, params
}

func (r *routeNode) calculateMaxParams() uint8 {
	if r == nil {
		return 0
	}
	var maxParams = uint8(0)
	if r.nType == param || r.nType == catchAll {
		maxParams = 1
	}
	for _, child := range r.children {
		params := child.calculateMaxParams()
		if params > maxParams {
			maxParams = params
		}
	}
	return maxParams + r.maxParams
}

// splitPath 分割路径
func splitPath(path string) []string {
	if path == "/" {
		return []string{}
	}
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}
