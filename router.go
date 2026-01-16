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
		// 修正：NewTire是routeNode的方法，需用空节点调用（原代码逻辑正确，但写法可优化）
		route = new(routeNode).NewTire()
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
	matchedNode, params := routeNode.FindChild(path)
	if matchedNode == nil || matchedNode.Handlers == nil {
		HTTPNotFound(c)
		return
	}
	// 绑定参数到Context
	for key, value := range params {
		c.SetParam(key, value)
	}
	// 执行处理器链
	for _, handler := range matchedNode.Handlers {
		handler(c)
	}
}

// routeNode 路由节点
type routeNode struct {
	path      string          // 节点的路径（公共前缀）
	indices   string          // 子节点首字符索引（去重）
	children  []*routeNode    // 子节点
	Handlers  HandleFuncChain // 处理器链
	priority  uint32          // 节点优先级
	nType     nodeType        // 节点类型
	maxParams uint8           // 子树中最大参数数量
	wildChild bool            // 是否有通配符子节点
	paramName string          // 参数节点名（:id → id；*path → path）
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
			switch part[0] {
			case ':':
				child.nType = param
				child.paramName = strings.TrimPrefix(part, ":")
			case '*':
				child.nType = catchAll
				child.wildChild = true
				child.paramName = strings.TrimPrefix(part, "*")
				// 通配符节点只能是最后一个节点，直接终止循环
				current.children = append(current.children, child)
				// 修正：indices去重后拼接
				if !strings.Contains(current.indices, part[0:1]) {
					current.indices += part[0:1]
				}
				current = child
				break
			default:
				child.nType = static
			}

			// 非通配符节点：正常添加
			if child.nType != catchAll {
				// 修正：indices避免重复字符
				if !strings.Contains(current.indices, part[0:1]) {
					current.indices += part[0:1]
				}
				current.children = append(current.children, child)
			}
		}

		// 更新当前节点的最大参数数
		current.maxParams = current.calculateMaxParams()
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

// FindChild 获取路由（核心修正：通配符匹配剩余所有片段）
func (r *routeNode) FindChild(path string) (*routeNode, map[string]string) {
	if r == nil || path == "" {
		return nil, nil
	}

	parts := splitPath(path)
	current := r
	params := make(map[string]string)
	idx := 0 // 记录当前遍历的片段索引

	for idx < len(parts) {
		part := parts[idx]
		if part == "" {
			idx++
			continue
		}

		var child *routeNode // 修正：初始化为nil，而非空结构体
		found := false

		if len(current.children) == 0 {
			return nil, nil
		}

		// 1. 优先精确匹配静态节点
		for _, c := range current.children {
			if c.path == part {
				child = c
				found = true
				break
			}
		}

		// 2. 匹配参数节点（:id）
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

		// 3. 匹配通配符节点（*path）→ 核心修正
		if !found {
			for _, c := range current.children {
				if c.nType == catchAll && c.wildChild {
					child = c
					found = true
					// 拼接剩余所有片段作为参数值（符合通配符语义）
					remainingParts := parts[idx:]
					paramValue := strings.Join(remainingParts, "/")
					// 设置参数名（兜底为*）
					paramName := c.paramName
					if paramName == "" {
						paramName = "*"
					}
					params[paramName] = paramValue
					// 通配符匹配剩余所有路径，直接终止遍历
					idx = len(parts)
					break
				}
			}
		}

		if !found {
			return nil, nil
		}

		current = child
		idx++
	}

	// 检查是否匹配到有效节点（通配符节点可能提前终止，需确认）
	if current == nil || (current.Handlers == nil && !current.wildChild) {
		return nil, nil
	}

	return current, params
}

// calculateMaxParams 计算子树最大参数数（修正逻辑错误）
func (r *routeNode) calculateMaxParams() uint8 {
	if r == nil {
		return 0
	}
	// 当前节点的参数数：参数/通配符节点算1，否则0
	currentParams := uint8(0)
	if r.nType == param || r.nType == catchAll {
		currentParams = 1
	}
	// 子节点的最大参数数
	maxChildParams := uint8(0)
	for _, child := range r.children {
		childParams := child.calculateMaxParams()
		if childParams > maxChildParams {
			maxChildParams = childParams
		}
	}
	// 总最大参数数 = 当前节点参数数 + 子节点最大参数数
	return currentParams + maxChildParams
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
