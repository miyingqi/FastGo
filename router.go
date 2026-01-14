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

// NewRouter 创建路由
func NewRouter() *Router {
	return &Router{
		route: make(map[string]*routeNode),
	}
}

// 获取路由
func (r *Router) getRoute(method string) *routeNode {
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
	if matchedNode == nil || matchedNode.Handlers == nil {
		HTTPNotFound(c)
		return
	}

	// 执行处理器链
	for _, handler := range matchedNode.Handlers {
		handler(c)
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

	// 分割路径
	parts := splitPath(path)
	current := r

	for _, part := range parts {
		if part == "" {
			continue
		}

		var child *routeNode

		// 检查是否已存在匹配的子节点
		for _, c := range current.children {
			if c.path == part {
				child = c
				break
			}
		}

		// 如果不存在，则创建新节点
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

			// 判断节点类型
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

	// 设置处理器链
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

		var child = &routeNode{}
		found := false

		if len(current.children) == 0 {
			return nil, nil
		}

		// 首先尝试精确匹配
		for _, c := range current.children {
			if c.path == part {
				child = c
				found = true
				break
			}
		}

		// 如果没找到精确匹配，尝试参数匹配
		if !found {
			for _, c := range current.children {
				if c.nType == param {
					child = c
					found = true
					// 存储参数值
					params[c.paramName] = part
					break
				}
			}
		}

		// 如果还没找到，尝试通配符匹配
		if !found {
			for _, c := range current.children {
				if c.nType == catchAll && c.wildChild {
					child = c
					found = true
					// 存储通配符参数
					params["*"] = part
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

// 分割路径
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
