package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/miyingqi/FastGo"
	"golang.org/x/time/rate"
)

// FileRangeReader 实现 RangeReader 接口，用于读取文件的指定范围
type FileRangeReader struct {
	filePath string
	fileInfo os.FileInfo
}

// Size 返回文件大小
func (fr *FileRangeReader) Size() int64 {
	return fr.fileInfo.Size()
}

// Name 返回文件名
func (fr *FileRangeReader) Name() string {
	return filepath.Base(fr.filePath)
}

// ContentType 返回文件 MIME 类型
func (fr *FileRangeReader) ContentType() string {
	return "application/octet-stream"
}

// RateLimitedReader 限速 Reader
type RateLimitedReader struct {
	reader  io.Reader
	limiter *rate.Limiter
}

func (r *RateLimitedReader) Read(p []byte) (n int, err error) {
	r.limiter.WaitN(context.Background(), len(p)) // 控制读取速率
	return r.reader.Read(p)
}

// ReadRange 读取文件的指定范围并限速
func (fr *FileRangeReader) ReadRange(ctx context.Context, start, end int64) (io.Reader, int64, error) {
	log.Printf("Opening file: %s, range: %d-%d", fr.filePath, start, end)

	file, err := os.Open(fr.filePath)
	if err != nil {
		log.Printf("Failed to open file %s: %v", fr.filePath, err)
		return nil, 0, err
	}
	defer file.Close() // 确保文件句柄最终关闭

	_, err = file.Seek(start, io.SeekStart)
	if err != nil {
		log.Printf("Failed to seek to position %d: %v", start, err)
		return nil, 0, err
	}

	// 创建限速 Reader（限制为 50MB/s）
	limiter := rate.NewLimiter(rate.Limit(50*1024*1024), 50*1024*1024) // 50MB/s
	reader := &RateLimitedReader{
		reader:  io.LimitReader(file, end-start+1),
		limiter: limiter,
	}

	log.Printf("Successfully created range reader for %d bytes", end-start+1)
	return reader, end - start + 1, nil
}

// 文件下载路由处理器
func downloadHandler(c *FastGo.Context) {
	log.Printf("Download request received from %s", c.ClientIP())

	// 指定要下载的文件路径
	filePath := "E:\\GO\\src\\FastGo\\examples\\range\\test_file.bin"
	log.Printf("Attempting to download file: %s", filePath)

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("File not found: %v", err)
		c.NotFound("File not found")
		return
	}

	log.Printf("File found: %s, size: %d bytes", fileInfo.Name(), fileInfo.Size())

	// 创建 RangeReader 实例
	reader := &FileRangeReader{
		filePath: filePath,
		fileInfo: fileInfo,
	}

	// 使用 ServeRange 处理断点续传
	log.Printf("Starting ServeRange for file: %s", fileInfo.Name())
	c.ServeRange(context.Background(), reader)
	log.Printf("ServeRange completed for file: %s", fileInfo.Name())
}

func main() {
	// 创建 FastGo 应用实例
	app := FastGo.NewFastGo()
	app.Router().GET("/download", downloadHandler)

	app.Run(":8080")
}
