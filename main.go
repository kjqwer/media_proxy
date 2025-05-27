package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

// Config 配置结构
type Config struct {
	Port      int    `yaml:"port"`
	BaseRoute string `yaml:"base_route"`
	MediaPath string `yaml:"media_path"`
}

// MediaServer 媒体服务器
type MediaServer struct {
	config     Config
	mediaFiles map[string]string // 路由 -> 文件路径
}

// 支持的媒体格式
var supportedFormats = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".webp": true,
	".mp4": true, ".avi": true, ".mkv": true, ".mov": true, ".wmv": true, ".flv": true,
	".webm": true, ".m4v": true, ".3gp": true, ".ts": true, ".m3u8": true,
}

// NewMediaServer 创建新的媒体服务器
func NewMediaServer() *MediaServer {
	return &MediaServer{
		mediaFiles: make(map[string]string),
	}
}

// LoadConfig 加载配置文件
func (ms *MediaServer) LoadConfig() error {
	configPath := "config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 创建默认配置文件
		defaultConfig := Config{
			Port:      8080,
			BaseRoute: "/media",
			MediaPath: "./media",
		}
		return ms.SaveConfig(defaultConfig)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, &ms.config)
}

// SaveConfig 保存配置文件
func (ms *MediaServer) SaveConfig(config Config) error {
	ms.config = config
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile("config.yaml", data, 0644)
}

// ScanMediaFiles 扫描媒体文件
func (ms *MediaServer) ScanMediaFiles() error {
	mediaPath := ms.config.MediaPath
	if mediaPath == "" {
		mediaPath = "./media"
	}

	// 检查media目录是否存在，不存在则创建
	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		log.Printf("媒体目录 %s 不存在，正在创建...", mediaPath)
		if err := os.MkdirAll(mediaPath, 0755); err != nil {
			return fmt.Errorf("创建媒体目录失败: %v", err)
		}
		log.Printf("媒体目录 %s 创建成功", mediaPath)
	}

	return filepath.Walk(mediaPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if supportedFormats[ext] {
			// 生成路由路径
			relPath, err := filepath.Rel(mediaPath, path)
			if err != nil {
				return err
			}

			// 将Windows路径分隔符转换为URL路径分隔符
			routePath := strings.ReplaceAll(relPath, "\\", "/")
			routePath = ms.config.BaseRoute + "/" + routePath

			ms.mediaFiles[routePath] = path
			log.Printf("发现媒体文件: %s -> %s", routePath, path)
		}

		return nil
	})
}

// SetupRoutes 设置路由
func (ms *MediaServer) SetupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 添加CORS中间件
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		method := c.Request.Method
		path := c.Request.URL.Path

		// 设置CORS头 - 必须在所有请求中设置
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, HEAD")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Range, Authorization, X-Requested-With, Cache-Control")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "false")
		c.Header("Access-Control-Max-Age", "86400") // 24小时预检缓存

		// 记录所有请求
		log.Printf("收到请求 - Method: %s, Path: %s, Origin: %s", method, path, origin)

		// 处理预检请求
		if method == "OPTIONS" {
			log.Printf("处理OPTIONS预检请求: %s, Origin: %s", path, origin)
			// 确保预检请求返回正确的状态码和头
			c.Header("Content-Length", "0")
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}

		// 记录跨域请求日志
		if origin != "" {
			log.Printf("CORS请求 - Origin: %s, Method: %s, Path: %s", origin, method, path)
		}

		c.Next()
	})

	// 添加请求日志中间件
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// 记录请求日志
		log.Printf("请求处理完成 - %s %s - 状态:%d - 耗时:%v - IP:%s",
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration, c.ClientIP())
	})

	// 添加健康检查端点
	r.GET("/health", func(c *gin.Context) {
		log.Printf("健康检查请求")
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"server": "media-proxy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// 使用一个统一的处理器来处理所有媒体相关请求
	r.GET(ms.config.BaseRoute+"/*filepath", ms.handleMediaRequest)

	return r
}

// handleMediaRequest 统一处理媒体请求
func (ms *MediaServer) handleMediaRequest(c *gin.Context) {
	filepath := c.Param("filepath")

	// 如果是请求文件列表
	if filepath == "/list" {
		ms.handleMediaList(c)
		return
	}

	// 否则处理媒体文件代理
	ms.handleMediaProxy(c)
}

// handleMediaList 处理媒体文件列表请求
func (ms *MediaServer) handleMediaList(c *gin.Context) {
	type MediaInfo struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
		Type string `json:"type"`
	}

	var mediaList []MediaInfo

	for routePath, filePath := range ms.mediaFiles {
		if info, err := os.Stat(filePath); err == nil {
			mediaType := "image"
			ext := strings.ToLower(filepath.Ext(filePath))
			if strings.Contains(".mp4.avi.mkv.mov.wmv.flv.webm.m4v.3gp.ts.m3u8", ext) {
				mediaType = "video"
			}

			mediaList = append(mediaList, MediaInfo{
				Path: routePath,
				Size: info.Size(),
				Type: mediaType,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    mediaList,
		"count":   len(mediaList),
	})
}

// handleMediaProxy 处理媒体文件代理请求
func (ms *MediaServer) handleMediaProxy(c *gin.Context) {
	requestPath := c.Request.URL.Path
	filePath, exists := ms.mediaFiles[requestPath]

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "文件未找到",
		})
		return
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "文件不存在",
		})
		return
	}

	// 设置Content-Type
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)

	// 处理Range请求（用于视频流式传输）
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		ms.handleRangeRequest(c, filePath, fileInfo.Size(), rangeHeader)
		return
	}

	// 普通文件传输
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Accept-Ranges", "bytes")

	file, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "无法打开文件",
		})
		return
	}
	defer file.Close()

	c.Status(http.StatusOK)
	io.Copy(c.Writer, file)
}

// handleRangeRequest 处理Range请求
func (ms *MediaServer) handleRangeRequest(c *gin.Context, filePath string, fileSize int64, rangeHeader string) {
	// 解析Range头
	re := regexp.MustCompile(`bytes=(\d*)-(\d*)`)
	matches := re.FindStringSubmatch(rangeHeader)

	if len(matches) != 3 {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	var start, end int64
	var err error

	if matches[1] != "" {
		start, err = strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	if matches[2] != "" {
		end, err = strconv.ParseInt(matches[2], 10, 64)
		if err != nil {
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	} else {
		end = fileSize - 1
	}

	// 验证范围
	if start < 0 || end >= fileSize || start > end {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1

	// 设置响应头
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Accept-Ranges", "bytes")

	// 设置Content-Type
	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)

	c.Status(http.StatusPartialContent)

	// 读取并发送指定范围的数据
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	file.Seek(start, 0)
	io.CopyN(c.Writer, file, contentLength)
}

func main() {
	server := NewMediaServer()

	// 加载配置
	if err := server.LoadConfig(); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("配置加载成功: 端口=%d, 基础路由=%s, 媒体路径=%s",
		server.config.Port, server.config.BaseRoute, server.config.MediaPath)

	// 扫描媒体文件
	if err := server.ScanMediaFiles(); err != nil {
		log.Fatalf("扫描媒体文件失败: %v", err)
	}

	log.Printf("扫描完成，发现 %d 个媒体文件", len(server.mediaFiles))

	if len(server.mediaFiles) == 0 {
		log.Printf("提示: 请将媒体文件放入 %s 目录中", server.config.MediaPath)
	}

	// 设置路由
	r := server.SetupRoutes()

	// 启动服务器
	addr := fmt.Sprintf(":%d", server.config.Port)
	log.Printf("媒体代理服务器启动在 http://localhost%s", addr)
	log.Printf("媒体文件目录: %s", server.config.MediaPath)
	log.Printf("API文档:")
	log.Printf("  获取媒体列表: GET %s/list", server.config.BaseRoute)
	log.Printf("  访问媒体文件: GET %s/<文件路径>", server.config.BaseRoute)

	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
