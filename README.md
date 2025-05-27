# 媒体代理服务器（自用）

一个用Go开发的媒体文件代理服务器，支持图片和视频的HTTP代理访问，特别支持视频的流式传输。

## 功能特性

- 自动扫描同目录下的所有媒体文件（图片、视频）
- 支持常见的图片格式：jpg, jpeg, png, gif, bmp, webp
- 支持常见的视频格式：mp4, avi, mkv, mov, wmv, flv, webm, m4v, 3gp, ts, m3u8
- 支持HTTP Range请求，实现视频流式传输
- 可通过配置文件自定义端口和路由
- 根据文件夹结构自动生成子路由
- 提供媒体文件列表API

## 使用方法

1. 运行 `build.bat` 构建可执行文件（可选）
2. 将 `media-proxy.exe` 放在包含媒体文件的目录中（可选）
3. 运行 `media-proxy.exe`
4. 服务器将在配置的端口启动（默认8080）

### 启动服务器

```bash
./media-proxy.exe
```

服务器将在配置的端口启动（默认8080），并自动扫描媒体文件。

## API 接口文档

### 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json` (API响应) / `对应媒体类型` (文件响应)
- **支持方法**: `GET`, `OPTIONS`

### 1. 获取媒体文件列表

获取所有可用的媒体文件列表。

**请求**
```http
GET /media/list
```

**响应示例**
```json
{
  "success": true,
  "data": [
    {
      "path": "/media/images/photo1.jpg",
      "size": 2048576,
      "type": "image"
    },
    {
      "path": "/media/videos/movie1.mp4",
      "size": 104857600,
      "type": "video"
    }
  ],
  "count": 2
}
```

**响应字段说明**
- `success`: 请求是否成功
- `data`: 媒体文件数组
  - `path`: 文件的API访问路径
  - `size`: 文件大小（字节）
  - `type`: 文件类型（"image" 或 "video"）
- `count`: 文件总数

### 2. 访问媒体文件

通过路径访问具体的媒体文件，支持完整下载和Range请求。

**请求**
```http
GET /media/{filepath}
```

**路径参数**
- `filepath`: 相对于media目录的文件路径

**请求示例**
```http
# 访问图片
GET /media/images/photo1.jpg

# 访问视频
GET /media/videos/movie1.mp4

# 访问子目录文件
GET /media/videos/action/film.mp4
```

**响应头**
```http
Content-Type: image/jpeg (根据文件类型)
Content-Length: 2048576
Accept-Ranges: bytes
Access-Control-Allow-Origin: *
```

### 3. Range请求（流式传输）

支持HTTP Range请求，用于视频分段加载和播放器拖拽定位。

**请求**
```http
GET /media/videos/movie1.mp4
Range: bytes=0-1023
```

**请求头说明**
- `Range`: 指定请求的字节范围
  - `bytes=0-1023`: 请求前1024字节
  - `bytes=1024-`: 从1024字节开始到文件末尾
  - `bytes=-1024`: 请求最后1024字节

**响应**
```http
HTTP/1.1 206 Partial Content
Content-Range: bytes 0-1023/104857600
Content-Length: 1024
Content-Type: video/mp4
```

**响应头说明**
- `206 Partial Content`: 部分内容响应
- `Content-Range`: 实际返回的字节范围和文件总大小
- `Content-Length`: 本次响应的数据长度

## 错误响应

### 文件未找到
```json
{
  "success": false,
  "message": "文件未找到"
}
```

### 文件不存在
```json
{
  "success": false,
  "message": "文件不存在"
}
```

### Range请求错误
```http
HTTP/1.1 416 Range Not Satisfiable
```

## 配置文件

`config.yaml` 文件用于配置服务器参数：

```yaml
port: 8080              # 服务器监听端口
base_route: "/media"    # API基础路由前缀
media_path: "./media"   # 媒体文件扫描目录
```

### 配置说明

- **port**: 服务器监听端口，默认8080
- **base_route**: API路由前缀，所有API都以此开头
- **media_path**: 媒体文件根目录，相对于可执行文件的路径


## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request来改进这个项目。 