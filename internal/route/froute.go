package route

import (
	"hash/fnv"
	"strings"
)

type nodeType uint8

const (
	static   nodeType = iota // 静态节点
	root                     // 根节点
	param                    // 参数节点，如 :id
	catchAll                 // 通配符节点，如 *path
)

// RouteNode 路由节点
type RouteNode struct {
	path      string              // 节点的路径（公共前缀）
	indices   map[byte]*RouteNode // 子节点首字符索引（仅静态节点）
	children  []*RouteNode        // 子节点
	handlers  string              // 存储路由hash
	priority  uint32              // 节点优先级
	nType     nodeType            // 节点类型
	maxParams uint8               // 子树中最大参数数量
	wildChild bool                // 是否有通配符子节点
	paramName string              // 参数节点名（:id → id；*path → path）
}

func (r *RouteNode) NewTire() *RouteNode {
	return &RouteNode{
		path:      "",
		indices:   make(map[byte]*RouteNode),
		children:  make([]*RouteNode, 0),
		handlers:  "",
		priority:  0,
		maxParams: 0,
		nType:     root,
		wildChild: false,
		paramName: "",
	}
}

// Insert 插入路由节点（修复通配符循环终止+优化节点查找）
func (r *RouteNode) Insert(path string) string {
	if r == nil || len(path) == 0 {
		return ""
	}

	hash := r.GetPathHash(path)
	parts := splitPath(path)
	current := r

	// 标记外层循环，用于通配符终止
outerLoop:
	for _, part := range parts {
		if part == "" {
			continue
		}

		var childNode *RouteNode
		// 优化：先遍历children查找（兼容所有节点类型）
		for _, child := range current.children {
			if child.path == part {
				childNode = child
				break
			}
		}

		// 如果不存在，则创建新节点
		if childNode == nil {
			childNode = &RouteNode{
				path:      part,
				indices:   make(map[byte]*RouteNode),
				children:  make([]*RouteNode, 0),
				handlers:  "",
				priority:  current.priority + 1,
				nType:     static,
				maxParams: 0,
				wildChild: false,
				paramName: "",
			}

			// 判断节点类型
			switch part[0] {
			case ':':
				childNode.nType = param
				childNode.paramName = strings.TrimPrefix(part, ":")
				current.addChild(childNode) // 参数节点直接添加
			case '*':
				childNode.nType = catchAll
				childNode.wildChild = true
				childNode.paramName = strings.TrimPrefix(part, "*")
				current.addChild(childNode) // 通配符节点添加
				current = childNode
				break outerLoop // 终止外层循环，确保通配符在末尾
			default:
				childNode.nType = static
				current.addChild(childNode) // 静态节点添加
			}
		}

		// 更新最大参数数
		current.maxParams = current.calculateMaxParams()
		current = childNode
	}
	// 将hash存入最终节点的handlers
	current.handlers = hash
	return hash
}

// addChild 添加子节点（仅静态节点更新indices，参数/通配符不更新）
func (r *RouteNode) addChild(child *RouteNode) {
	if r == nil || child == nil {
		return
	}
	// 添加到children数组
	r.children = append(r.children, child)

	// 仅静态节点更新indices（参数/通配符节点不参与indices索引）
	if child.nType == static && len(child.path) > 0 {
		if r.indices == nil {
			r.indices = make(map[byte]*RouteNode)
		}
		r.indices[child.path[0]] = child
	}
}

// FindChild 修复参数提取逻辑：优先静态匹配，再参数，最后通配符
func (r *RouteNode) FindChild(path string) (*RouteNode, map[string]string) {
	if r == nil || path == "" {
		return nil, nil
	}

	parts := splitPath(path)
	current := r
	params := make(map[string]string, current.maxParams)
	idx := 0

	for idx < len(parts) {
		part := parts[idx]
		if part == "" {
			idx++
			continue
		}

		var child *RouteNode
		found := false

		if len(current.children) == 0 {
			return nil, nil
		}

		// 1. 优先匹配静态节点（仅静态节点在indices中）
		if current.indices != nil {
			if staticChild, exists := current.indices[part[0]]; exists && staticChild.path == part {
				child = staticChild
				found = true
			}
		}

		// 2. 静态匹配失败 → 遍历所有子节点找静态节点（兜底）
		if !found {
			for _, c := range current.children {
				if c.nType == static && c.path == part {
					child = c
					found = true
					break
				}
			}
		}

		// 3. 静态匹配失败 → 匹配参数节点（核心修复：正确命中:file）
		if !found {
			for _, c := range current.children {
				if c.nType == param {
					child = c
					found = true
					// 提取参数值：请求片段（如test.txt）赋值给参数名（如file）
					params[c.paramName] = part
					break
				}
			}
		}

		// 4. 参数匹配失败 → 匹配通配符节点
		if !found {
			for _, c := range current.children {
				if c.nType == catchAll && c.wildChild {
					child = c
					found = true
					remainingParts := parts[idx:]
					paramValue := strings.Join(remainingParts, "/")
					paramName := c.paramName
					if paramName == "" {
						paramName = "*"
					}
					params[paramName] = paramValue
					idx = len(parts) // 终止遍历
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

	// 检查节点有效性
	if current == nil || current.handlers == "" {
		return nil, nil
	}

	return current, params
}

// calculateMaxParams 计算子树最大参数数
func (r *RouteNode) calculateMaxParams() uint8 {
	if r == nil {
		return 0
	}
	currentParams := uint8(0)
	if r.nType == param || r.nType == catchAll {
		currentParams = 1
	}
	maxChildParams := uint8(0)
	for _, child := range r.children {
		childParams := child.calculateMaxParams()
		if childParams > maxChildParams {
			maxChildParams = childParams
		}
	}
	return currentParams + maxChildParams
}

// GetPathHash 生成路由hash
func (r *RouteNode) GetPathHash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(path))
	return string([]byte{
		hexChar((h.Sum32() >> 28) & 0x0F),
		hexChar((h.Sum32() >> 24) & 0x0F),
		hexChar((h.Sum32() >> 20) & 0x0F),
		hexChar((h.Sum32() >> 16) & 0x0F),
		hexChar((h.Sum32() >> 12) & 0x0F),
		hexChar((h.Sum32() >> 8) & 0x0F),
		hexChar((h.Sum32() >> 4) & 0x0F),
		hexChar(h.Sum32() & 0x0F),
	})
}

// GetHashs 获取节点hash
func (r *RouteNode) GetHashs() string {
	return r.handlers
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

// hexChar 数字转16进制字符
func hexChar(n uint32) byte {
	if n < 10 {
		return '0' + byte(n)
	}
	return 'a' + byte(n-10)
}
