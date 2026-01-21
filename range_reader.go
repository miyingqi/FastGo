package FastGo

import (
	"context"
	"io"
)

// RangeReader 断点续传数据源通用接口
// 任意存储类型（本地文件/MinIO/OSS/内存）只需实现此接口，即可支持断点续传
type RangeReader interface {
	// Size 返回数据总大小（字节）
	Size() int64

	// ReadRange 读取指定范围的数据 [start, end]（包含end）
	// 返回：数据读取器、读取长度、错误
	ReadRange(ctx context.Context, start, end int64) (io.Reader, int64, error)

	// Name 返回数据名称（用于下载时的Content-Disposition）
	Name() string

	// ContentType 返回数据的MIME类型（如application/pdf）
	ContentType() string
}
