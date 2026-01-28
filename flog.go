package FastGo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// LogLevel 定义日志级别
type LogLevel uint8

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	FATAL
)

var levelStrings = [5]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorCyan    = "\033[36m"
	ColorBold    = "\033[1m"
	ColorBoldRed = "\033[1;31m"
)

var levelColors = [5]string{
	DEBUG:   ColorCyan,
	INFO:    ColorGreen,
	WARNING: ColorYellow,
	ERROR:   ColorRed,
	FATAL:   ColorBoldRed,
}

// colorManager 颜色管理器，处理不同环境的颜色支持
type colorManager struct {
	supportsColor bool
}

// NewColorManager 创建新的颜色管理器
func newColorManager() *colorManager {
	cm := &colorManager{}
	cm.detectColorSupport()
	return cm
}

// detectColorSupport 检测当前环境是否支持颜色
func (cm *colorManager) detectColorSupport() {
	if runtime.GOOS == "windows" {
		cm.supportsColor = true // 简化处理
	} else {
		cm.supportsColor = cm.isTerminal(os.Stdout) &&
			os.Getenv("TERM") != "dumb" &&
			os.Getenv("NO_COLOR") == ""
	}
}

// isTerminal 检查是否是终端
func (cm *colorManager) isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty(f.Fd())
	}
	return false
}

// isatty 检查文件描述符是否是TTY
func isatty(fd uintptr) bool {
	return fd == 1 || fd == 2 // stdout/stderr通常为1/2
}

// ApplyColor 应用颜色到文本
func (cm *colorManager) ApplyColor(text, colorCode string, enableColor bool) string {
	if !cm.supportsColor || !enableColor {
		return text
	}
	return colorCode + text + ColorReset
}

// ApplyLevelColor 应用级别颜色
func (cm *colorManager) ApplyLevelColor(level LogLevel, enableColor bool) string {
	if !cm.supportsColor || !enableColor {
		return levelStrings[level]
	}
	return levelColors[level] + levelStrings[level] + ColorReset
}

// timeCache 缓存时间戳，减少时间调用开销
type timeCache struct {
	timestamp atomic.Value // 存储格式化的时间字符串
}

func newTimeCache() *timeCache {
	tc := &timeCache{}
	tc.update()
	go tc.refreshLoop()
	return tc
}

func (tc *timeCache) update() {
	now := time.Now()
	formatted := now.Format("2006-01-02 15:04:05")
	tc.timestamp.Store(formatted)
}

func (tc *timeCache) Get() string {
	return tc.timestamp.Load().(string)
}

func (tc *timeCache) refreshLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		tc.update()
	}
}

// LogRecord 日志记录结构
type LogRecord struct {
	Level       LogLevel
	Message     string
	Fields      map[string]interface{}
	Time        time.Time
	EnableColor bool
	Module      string // 模块名称，用于区分日志来源
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level       LogLevel
	Output      io.Writer
	BufferSize  int
	WorkerCount int
	EnableColor bool
}

// defaultConfig 默认配置
var defaultConfig = LoggerConfig{
	Level:       INFO,
	BufferSize:  1000,
	WorkerCount: 2,
	EnableColor: true,
	Output:      os.Stdout,
}

// AsyncLogger 异步日志记录器
type AsyncLogger struct {
	level        int32
	output       io.Writer
	queue        chan LogRecord
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	workerCount  int
	timeCache    *timeCache
	bufPool      sync.Pool
	enableColor  bool
	closed       int32 // 原子操作标记是否已关闭
	colorManager *colorManager
}

// NewAsyncLogger 创建异步日志记录器
func NewAsyncLogger(config LoggerConfig) *AsyncLogger {
	if config.Output == nil {
		config.Output = defaultConfig.Output
	}
	if config.BufferSize <= 0 {
		config.BufferSize = defaultConfig.BufferSize
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = defaultConfig.WorkerCount
	}

	ctx, cancel := context.WithCancel(context.Background())

	logger := &AsyncLogger{
		level:        int32(config.Level),
		output:       config.Output,
		queue:        make(chan LogRecord, config.BufferSize),
		workerCount:  config.WorkerCount,
		timeCache:    newTimeCache(),
		enableColor:  config.EnableColor,
		colorManager: newColorManager(),
		bufPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 1024))
			},
		},
		ctx:    ctx,
		cancel: cancel,
	}

	logger.startWorkers()
	return logger
}

// NewAsyncLoggerDF 使用默认配置创建异步日志记录器
func NewAsyncLoggerDF() *AsyncLogger {
	return NewAsyncLogger(defaultConfig)
}

// NewAsyncLoggerSP 快速创建日志器（使用默认配置，可选级别）
func NewAsyncLoggerSP(level LogLevel) *AsyncLogger {
	config := defaultConfig
	config.Level = level
	return NewAsyncLogger(config)
}

// WithLevel 设置日志级别
func (al *AsyncLogger) WithLevel(level LogLevel) *AsyncLogger {
	al.SetLevel(level)
	return al
}

// WithOutput 设置输出目标
func (al *AsyncLogger) WithOutput(output io.Writer) *AsyncLogger {
	al.output = output
	return al
}

// WithColor 设置是否启用颜色
func (al *AsyncLogger) WithColor(enable bool) *AsyncLogger {
	al.enableColor = enable
	return al
}

// startWorkers 启动工作协程
func (al *AsyncLogger) startWorkers() {
	for i := 0; i < al.workerCount; i++ {
		al.wg.Add(1)
		go al.worker(i)
	}
}

// worker 工作协程
func (al *AsyncLogger) worker(id int) {
	defer al.wg.Done()

	for {
		select {
		case record, ok := <-al.queue:
			if !ok {
				return // 通道已关闭
			}
			al.processRecord(record)
		case <-al.ctx.Done():
			return // 上下文被取消
		}
	}
}

// processRecord 处理单条日志记录
func (al *AsyncLogger) processRecord(record LogRecord) {
	buf := al.bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		al.bufPool.Put(buf)
	}()

	timestamp := record.Time.Format("2006-01-02 15:04:05")

	buf.WriteString("[")
	buf.WriteString(timestamp)
	buf.WriteString("] [")

	levelText := al.colorManager.ApplyLevelColor(record.Level, record.EnableColor && al.enableColor)
	buf.WriteString(levelText)
	buf.WriteString("] ")

	// 如果有模块名，则在日志中显示模块名
	if record.Module != "" {
		buf.WriteString("[")
		moduleText := al.colorManager.ApplyColor(record.Module, ColorCyan, record.EnableColor && al.enableColor)
		buf.WriteString(moduleText)
		buf.WriteString("] ")
	}

	buf.WriteString(record.Message)

	if len(record.Fields) > 0 {
		buf.WriteString(" |")
		for k, v := range record.Fields {
			buf.WriteString(" ")

			keyText := al.colorManager.ApplyColor(k, ColorBold, record.EnableColor && al.enableColor)
			buf.WriteString(keyText)
			buf.WriteString("=")
			buf.WriteString(formatValue(v))
		}
	}

	buf.WriteString("\n")

	_, err := al.output.Write(buf.Bytes())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
	}
}

// formatValue 格式化值为字符串
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case error:
		return val.Error()
	case fmt.Stringer:
		return val.String()
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	case []string:
		return strings.Join(val, ", ")
	case []int:
		strs := make([]string, len(val))
		for i, v := range val {
			strs[i] = fmt.Sprintf("%d", v)
		}
		return strings.Join(strs, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// isEnabled 检查是否启用该日志级别
func (al *AsyncLogger) isEnabled(level LogLevel) bool {
	return level >= LogLevel(atomic.LoadInt32(&al.level))
}

// enqueue 添加日志记录到队列
func (al *AsyncLogger) enqueue(level LogLevel, msg string, fields map[string]interface{}, enableColor bool) {
	al.enqueueWithModule(level, msg, "", fields, enableColor)
}

// enqueueWithModule 添加带模块名的日志记录到队列
func (al *AsyncLogger) enqueueWithModule(level LogLevel, msg, module string, fields map[string]interface{}, enableColor bool) {
	if !al.isEnabled(level) || atomic.LoadInt32(&al.closed) == 1 {
		return
	}

	record := LogRecord{
		Level:       level,
		Message:     msg,
		Fields:      fields,
		Time:        time.Now(),
		EnableColor: enableColor,
		Module:      module,
	}

	select {
	case al.queue <- record:
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Log queue full, dropping log: %s\n", msg)
	}
}

// SetLevel 设置日志级别
func (al *AsyncLogger) SetLevel(level LogLevel) {
	atomic.StoreInt32(&al.level, int32(level))
}

// GetLevel 获取当前日志级别
func (al *AsyncLogger) GetLevel() LogLevel {
	return LogLevel(atomic.LoadInt32(&al.level))
}

// Debug 异步记录调试日志
func (al *AsyncLogger) Debug(msg string, fields ...map[string]interface{}) {
	al.enqueue(DEBUG, msg, mergeFields(fields...), al.enableColor)
}

// Info 异步记录信息日志
func (al *AsyncLogger) Info(msg string, fields ...map[string]interface{}) {
	al.enqueue(INFO, msg, mergeFields(fields...), al.enableColor)
}

// Warning 异步记录警告日志
func (al *AsyncLogger) Warning(msg string, fields ...map[string]interface{}) {
	al.enqueue(WARNING, msg, mergeFields(fields...), al.enableColor)
}

// Error 异步记录错误日志
func (al *AsyncLogger) Error(msg string, fields ...map[string]interface{}) {
	al.enqueue(ERROR, msg, mergeFields(fields...), al.enableColor)
}

// Fatal 异步记录致命错误日志并退出
func (al *AsyncLogger) Fatal(msg string, fields ...map[string]interface{}) {
	al.enqueue(FATAL, msg, mergeFields(fields...), al.enableColor)
	al.Flush()
	os.Exit(1)
}

// DebugWithModule 异步记录带模块名的调试日志
func (al *AsyncLogger) DebugWithModule(module, msg string, fields ...map[string]interface{}) {
	al.enqueueWithModule(DEBUG, msg, module, mergeFields(fields...), al.enableColor)
}

// InfoWithModule 异步记录带模块名的信息日志
func (al *AsyncLogger) InfoWithModule(module, msg string, fields ...map[string]interface{}) {
	al.enqueueWithModule(INFO, msg, module, mergeFields(fields...), al.enableColor)
}

// WarningWithModule 异步记录带模块名的警告日志
func (al *AsyncLogger) WarningWithModule(module, msg string, fields ...map[string]interface{}) {
	al.enqueueWithModule(WARNING, msg, module, mergeFields(fields...), al.enableColor)
}

// ErrorWithModule 异步记录带模块名的错误日志
func (al *AsyncLogger) ErrorWithModule(module, msg string, fields ...map[string]interface{}) {
	al.enqueueWithModule(ERROR, msg, module, mergeFields(fields...), al.enableColor)
}

// FatalWithModule 异步记录带模块名的致命错误日志并退出
func (al *AsyncLogger) FatalWithModule(module, msg string, fields ...map[string]interface{}) {
	al.enqueueWithModule(FATAL, msg, module, mergeFields(fields...), al.enableColor)
	al.Flush()
	os.Exit(1)
}

// Sync 等待所有日志处理完成
func (al *AsyncLogger) Sync() {
	queueLen := len(al.queue)
	if queueLen == 0 {
		return
	}

	start := time.Now()
	for len(al.queue) > 0 {
		if time.Since(start) > time.Second*5 {
			break // 超时保护
		}
		time.Sleep(time.Millisecond * 10)
	}
}

// Flush 等待所有日志处理完成
func (al *AsyncLogger) Flush() {
	al.Sync()
}

// Close 关闭异步日志记录器
func (al *AsyncLogger) Close() error {
	if !atomic.CompareAndSwapInt32(&al.closed, 0, 1) {
		return nil // 已经关闭
	}

	al.cancel()
	close(al.queue)
	al.wg.Wait()
	return nil
}

// mergeFields 合并多个字段映射
func mergeFields(maps ...map[string]interface{}) map[string]interface{} {
	if len(maps) == 0 {
		return nil
	}

	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}

	return result
}

// FileWriter 文件写入器
type FileWriter struct {
	filePath string
	file     *os.File
	mu       sync.Mutex
	maxSize  int64 // 最大文件大小，单位字节
}

// NewFileWriter 创建文件写入器
func NewFileWriter(filePath string, maxSize int64) (*FileWriter, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	writer := &FileWriter{
		filePath: filePath,
		file:     file,
		maxSize:  maxSize,
	}

	return writer, nil
}

// Write 写入数据到文件
func (fw *FileWriter) Write(p []byte) (n int, err error) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.maxSize > 0 {
		currentSize := int64(len(p))
		if stat, err := fw.file.Stat(); err == nil {
			currentSize += stat.Size()
		}

		if currentSize > fw.maxSize {
			if err := fw.rotate(); err != nil {
				return 0, err
			}
		}
	}

	return fw.file.Write(p)
}

// rotate 轮转日志文件
func (fw *FileWriter) rotate() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if err := fw.file.Close(); err != nil {
		return fmt.Errorf("failed to close current log file: %v", err)
	}

	backupName := fmt.Sprintf("%s.%s", fw.filePath, time.Now().Format("2006-01-02T15-04-05"))
	if err := os.Rename(fw.filePath, backupName); err != nil {
		file, err2 := os.OpenFile(fw.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err2 != nil {
			return fmt.Errorf("failed to reopen original file after rotation failure: %v, original error: %v", err2, err)
		}
		fw.file = file
		return fmt.Errorf("failed to rotate log file: %v", err)
	}

	file, err := os.OpenFile(fw.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %v", err)
	}

	fw.file = file
	return nil
}

// Close 关闭文件写入器
func (fw *FileWriter) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.file.Close()
}

// MultiWriter 支持同时写入多个输出的目标
type MultiWriter struct {
	writers []io.Writer
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err := w.Write(p)
		if err != nil {
			return n, err
		}
		if n != len(p) {
			return n, io.ErrShortWrite
		}
	}
	return len(p), nil
}

// FileLogger 文件日志记录器
type FileLogger struct {
	asyncLogger *AsyncLogger
	fileWriter  *FileWriter
}

// FileLoggerConfig 文件日志配置
type FileLoggerConfig struct {
	Level       LogLevel
	FilePath    string
	MaxFileSize int64
	EnableColor bool
}

// DefaultFileConfig 默认文件日志配置
var DefaultFileConfig = FileLoggerConfig{
	Level:       INFO,
	FilePath:    "core.log",
	MaxFileSize: 10 * 1024 * 1024, // 10MB
	EnableColor: true,
}

// NewFileLogger 创建文件日志记录器
func NewFileLogger(config FileLoggerConfig) (*FileLogger, error) {
	fileWriter, err := NewFileWriter(config.FilePath, config.MaxFileSize)
	if err != nil {
		return nil, err
	}

	var output io.Writer
	if config.EnableColor {
		output = NewMultiWriter(fileWriter, os.Stdout)
	} else {
		output = fileWriter
	}

	asyncLogger := NewAsyncLogger(LoggerConfig{
		Level:       config.Level,
		Output:      output,
		BufferSize:  defaultConfig.BufferSize,
		WorkerCount: defaultConfig.WorkerCount,
		EnableColor: config.EnableColor,
	})

	logger := &FileLogger{
		asyncLogger: asyncLogger,
		fileWriter:  fileWriter,
	}

	return logger, nil
}

// NewFileLoggerWithDefaults 使用默认配置创建文件日志记录器
func NewFileLoggerWithDefaults() (*FileLogger, error) {
	return NewFileLogger(DefaultFileConfig)
}

// NewFileLoggerSimple 快速创建文件日志器（使用默认配置，可选级别和文件路径）
func NewFileLoggerSimple(level LogLevel, filePath string) (*FileLogger, error) {
	config := DefaultFileConfig
	config.Level = level
	config.FilePath = filePath
	return NewFileLogger(config)
}

// NewConsoleAndFileLogger 创建同时输出到控制台和文件的日志记录器
func NewConsoleAndFileLogger(config FileLoggerConfig) (*FileLogger, error) {
	fileWriter, err := NewFileWriter(config.FilePath, config.MaxFileSize)
	if err != nil {
		return nil, err
	}

	multiWriter := NewMultiWriter(os.Stdout, fileWriter)
	asyncLogger := NewAsyncLogger(LoggerConfig{
		Level:       config.Level,
		Output:      multiWriter,
		BufferSize:  defaultConfig.BufferSize,
		WorkerCount: defaultConfig.WorkerCount,
		EnableColor: config.EnableColor,
	})

	logger := &FileLogger{
		asyncLogger: asyncLogger,
		fileWriter:  fileWriter,
	}

	return logger, nil
}

// NewConsoleAndFileLoggerWithDefaults 使用默认配置创建控制台和文件日志记录器
func NewConsoleAndFileLoggerWithDefaults() (*FileLogger, error) {
	return NewConsoleAndFileLogger(DefaultFileConfig)
}

// SetLevel 方法代理到内部的异步日志记录器
func (fl *FileLogger) SetLevel(level LogLevel) {
	fl.asyncLogger.SetLevel(level)
}

func (fl *FileLogger) GetLevel() LogLevel {
	return fl.asyncLogger.GetLevel()
}

func (fl *FileLogger) WithLevel(level LogLevel) *FileLogger {
	fl.SetLevel(level)
	return fl
}

func (fl *FileLogger) WithColor(enable bool) *FileLogger {
	fl.asyncLogger.enableColor = enable
	return fl
}

func (fl *FileLogger) Debug(msg string, fields ...map[string]interface{}) {
	fl.asyncLogger.Debug(msg, fields...)
}

func (fl *FileLogger) Info(msg string, fields ...map[string]interface{}) {
	fl.asyncLogger.Info(msg, fields...)
}

func (fl *FileLogger) Warning(msg string, fields ...map[string]interface{}) {
	fl.asyncLogger.Warning(msg, fields...)
}

func (fl *FileLogger) Error(msg string, fields ...map[string]interface{}) {
	fl.asyncLogger.Error(msg, fields...)
}

func (fl *FileLogger) Fatal(msg string, fields ...map[string]interface{}) {
	fl.asyncLogger.Fatal(msg, fields...)
}

func (fl *FileLogger) Sync() {
	fl.asyncLogger.Sync()
}

func (fl *FileLogger) Close() error {
	err1 := fl.asyncLogger.Close()
	err2 := fl.fileWriter.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// SafeFormat 安全格式化字符串，防止nil指针错误
func SafeFormat(format string, args ...interface{}) string {
	if format == "" {
		if len(args) == 0 {
			return ""
		}
		return fmt.Sprint(args...)
	}

	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Format error: %v\n", r)
		}
	}()

	return fmt.Sprintf(format, args...)
}

// IsColorSupported 检查当前环境是否支持颜色
func IsColorSupported() bool {
	return newColorManager().supportsColor
}

// GetLevelColor 获取指定级别的颜色代码
func GetLevelColor(level LogLevel) string {
	return levelColors[level]
}

// ApplyColorToText 给文本应用颜色
func ApplyColorToText(text, colorCode string) string {
	cm := newColorManager()
	if !cm.supportsColor {
		return text
	}
	return colorCode + text + ColorReset
}

// SyncLogger 同步日志记录器
type SyncLogger struct {
	level        int32
	output       io.Writer
	timeCache    *timeCache
	mutex        sync.Mutex
	bufPool      sync.Pool
	enableColor  bool
	colorManager *colorManager
}

// NewSyncLogger 创建同步日志记录器
func NewSyncLogger(level LogLevel) *SyncLogger {
	config := defaultConfig
	config.Level = level
	return NewSyncLoggerWithConfig(config)
}

// NewSyncLoggerWithConfig 使用配置创建同步日志记录器
func NewSyncLoggerWithConfig(config LoggerConfig) *SyncLogger {
	if config.Output == nil {
		config.Output = defaultConfig.Output
	}

	logger := &SyncLogger{
		level:        int32(config.Level),
		output:       config.Output,
		timeCache:    newTimeCache(),
		enableColor:  config.EnableColor,
		colorManager: newColorManager(),
		bufPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 1024))
			},
		},
	}

	return logger
}

// processRecord 同步处理单条日志记录
func (sl *SyncLogger) processRecord(record LogRecord) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	buf := sl.bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		sl.bufPool.Put(buf)
	}()

	timestamp := record.Time.Format("2006-01-02 15:04:05")

	buf.WriteString("[")
	buf.WriteString(timestamp)
	buf.WriteString("] [")

	levelText := sl.colorManager.ApplyLevelColor(record.Level, record.EnableColor && sl.enableColor)
	buf.WriteString(levelText)
	buf.WriteString("] ")

	// 如果有模块名，则在日志中显示模块名
	if record.Module != "" {
		buf.WriteString("[")
		moduleText := sl.colorManager.ApplyColor(record.Module, ColorCyan, record.EnableColor && sl.enableColor)
		buf.WriteString(moduleText)
		buf.WriteString("] ")
	}

	buf.WriteString(record.Message)

	if len(record.Fields) > 0 {
		buf.WriteString(" |")
		for k, v := range record.Fields {
			buf.WriteString(" ")

			keyText := sl.colorManager.ApplyColor(k, ColorBold, record.EnableColor && sl.enableColor)
			buf.WriteString(keyText)
			buf.WriteString("=")
			buf.WriteString(formatValue(v))
		}
	}

	buf.WriteString("\n")

	_, err := sl.output.Write(buf.Bytes())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
	}
}

// isEnabled 检查是否启用该日志级别
func (sl *SyncLogger) isEnabled(level LogLevel) bool {
	return level >= LogLevel(atomic.LoadInt32(&sl.level))
}

// enqueue 同步添加日志记录
func (sl *SyncLogger) enqueue(level LogLevel, msg string, fields map[string]interface{}, enableColor bool) {
	sl.enqueueWithModule(level, msg, "", fields, enableColor)
}

// enqueueWithModule 同步添加带模块名的日志记录
func (sl *SyncLogger) enqueueWithModule(level LogLevel, msg, module string, fields map[string]interface{}, enableColor bool) {
	if !sl.isEnabled(level) {
		return
	}

	record := LogRecord{
		Level:       level,
		Message:     msg,
		Fields:      fields,
		Time:        time.Now(),
		EnableColor: enableColor,
		Module:      module,
	}

	sl.processRecord(record) // 直接同步处理
}

// SetLevel 设置日志级别
func (sl *SyncLogger) SetLevel(level LogLevel) {
	atomic.StoreInt32(&sl.level, int32(level))
}

// GetLevel 获取当前日志级别
func (sl *SyncLogger) GetLevel() LogLevel {
	return LogLevel(atomic.LoadInt32(&sl.level))
}

// Debug 记录调试日志
func (sl *SyncLogger) Debug(msg string, fields ...map[string]interface{}) {
	sl.enqueue(DEBUG, msg, mergeFields(fields...), sl.enableColor)
}

// Info 记录信息日志
func (sl *SyncLogger) Info(msg string, fields ...map[string]interface{}) {
	sl.enqueue(INFO, msg, mergeFields(fields...), sl.enableColor)
}

// Warning 记录警告日志
func (sl *SyncLogger) Warning(msg string, fields ...map[string]interface{}) {
	sl.enqueue(WARNING, msg, mergeFields(fields...), sl.enableColor)
}

// Error 记录错误日志
func (sl *SyncLogger) Error(msg string, fields ...map[string]interface{}) {
	sl.enqueue(ERROR, msg, mergeFields(fields...), sl.enableColor)
}

// Fatal 记录致命错误日志并退出
func (sl *SyncLogger) Fatal(msg string, fields ...map[string]interface{}) {
	sl.enqueue(FATAL, msg, mergeFields(fields...), sl.enableColor)
	os.Exit(1)
}

// DebugWithModule 记录带模块名的调试日志
func (sl *SyncLogger) DebugWithModule(module, msg string, fields ...map[string]interface{}) {
	sl.enqueueWithModule(DEBUG, msg, module, mergeFields(fields...), sl.enableColor)
}

// InfoWithModule 记录带模块名的信息日志
func (sl *SyncLogger) InfoWithModule(module, msg string, fields ...map[string]interface{}) {
	sl.enqueueWithModule(INFO, msg, module, mergeFields(fields...), sl.enableColor)
}

// WarningWithModule 记录带模块名的警告日志
func (sl *SyncLogger) WarningWithModule(module, msg string, fields ...map[string]interface{}) {
	sl.enqueueWithModule(WARNING, msg, module, mergeFields(fields...), sl.enableColor)
}

// ErrorWithModule 记录带模块名的错误日志
func (sl *SyncLogger) ErrorWithModule(module, msg string, fields ...map[string]interface{}) {
	sl.enqueueWithModule(ERROR, msg, module, mergeFields(fields...), sl.enableColor)
}

// FatalWithModule 记录带模块名的致命错误日志并退出
func (sl *SyncLogger) FatalWithModule(module, msg string, fields ...map[string]interface{}) {
	sl.enqueueWithModule(FATAL, msg, module, mergeFields(fields...), sl.enableColor)
	os.Exit(1)
}
