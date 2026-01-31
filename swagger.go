package FastGo

// SwaggerHandler 处理 Swagger UI 请求
func SwaggerHandler() HandlerFunc {
	return func(c ContextInterface) {
		if c.Path() == "/swagger/doc.json" {
			// 返回生成的 swagger.json
			c.SetHeader("Content-Type", "application/json")
			c.Data(200, "application/json", []byte(Docs))
			return
		}

		if c.Path() == "/swagger/" || c.Path() == "/swagger" {
			// 重定向到 index.html
			c.Redirect(302, "/swagger/index.html")
			return
		}

		if c.Path() == "/swagger/index.html" {
			c.SetHeader("Content-Type", "text/html")
			c.Data(200, "text/html", []byte(SwaggerIndexHTML))
			return
		}

		// 返回 404
		c.NotFound("Not Found")
	}
}

// Docs 是从 swag 生成的文档内容
// 这个变量应该在 docs 包中定义
var Docs string

// SwaggerIndexHTML 是 Swagger UI 的 HTML 页面
const SwaggerIndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@4/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@4/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@4/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: "/swagger/doc.json",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
            window.ui = ui;
        };
    </script>
</body>
</html>`
