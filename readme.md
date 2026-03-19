# Waybar MPRIS Lyrics 🎵

一个轻量级的 Go 程序，用于获取和显示 Linux 上播放中的音乐歌词。通过 D-Bus MPRIS 接口直接读取播放器信息，支持多个音乐数据源。

## ✨ 功能特性

- **D-Bus MPRIS 集成**：直接通过 D-Bus 读取播放器元数据，无需外部依赖
- **多源歌词查询**：
  - 🎵 **网易云音乐 API** - 优先级最高，支持中英文歌词
  - 🔗 **Lrclib API** - 备用方案，适合网易云找不到的歌曲
- **播放器支持**：
  - 优先支持 **SPlayer**（提取直接歌曲 ID）
  - 支持所有 MPRIS 标准兼容播放器
- **智能搜索**：
  - SPlayer 自动提取歌曲 ID
  - 其他播放器通过 标题 + 艺术家 组合搜索
- **元数据解析**：标题、艺术家、专辑、时长、封面 URL 等

## 📋 系统要求

- **操作系统**：Linux (需要 D-Bus 支持)
- **Go 版本**：1.20 或更高
- **依赖**：
  - `github.com/godbus/dbus/v5` - D-Bus 客户端库
  - `golang.org/x/sys` - 系统库

## 🚀 快速开始

### 安装

克隆仓库：

```bash
git clone https://github.com/yourusername/waybar-MPRIS-lyrics.git
cd waybar-MPRIS-lyrics
```

获取依赖：

```bash
go get -u github.com/godbus/dbus/v5
```

编译：

```bash
go build -o lyrics main.go
```

### 使用

运行程序：

```bash
./lyrics
```

**输出示例：**

```
播放器: splayer.instance19027
标题: 马来个福
艺术家: 洛天依/青森灼柳
专辑: 马来个福
搜索得到的歌曲ID: 3349609304

=== 原歌词 ===
[00:18.684]我们都是情场老手
[00:22.657]你和我都知道爱情的规则
...

=== 翻译歌词 ===
[00:18.684]We're no strangers to love
[00:22.657]You know the rules and so do I
...
```

## 🏗️ 工作流程

```
┌─────────────────────────────────┐
│  连接 D-Bus Session Bus          │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  查找 MPRIS 播放器              │
│  (优先: SPlayer)                │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  获取播放器元数据                │
│  (标题、艺术家、专辑、时长等)    │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  SPlayer?                       │
│  ├─ 是 → 直接提取歌曲 ID         │
│  └─ 否 → 网易云 API 搜索        │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  搜索成功?                       │
│  ├─ 是 → 获取网易云歌词         │
│  └─ 否 → 尝试 Lrclib API        │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  显示歌词                        │
│  (原歌词 + 翻译歌词)            │
└─────────────────────────────────┘
```

## 📁 项目结构

```
waybar-MPRIS-lyrics/
├── main.go              # 主程序
├── go.mod              # Go 模块定义
├── go.sum              # 依赖校验
├── readme.md           # 项目文档
└── LICENSE             # 许可证
```

## 🔧 API 端点

### 网易云音乐 API

**搜索歌曲**

```
GET https://music.163.com/api/search/get/web
参数: s={query}&type=1&limit=1
```

**获取歌词**

```
GET https://music.163.com/api/song/lyric
参数: id={songId}&lv=1&tv=1
```

### Lrclib API

**查询歌词**

```
GET https://lrclib.net/api/get
参数: track_name={title}&artist_name={artist}&album_name={album}&duration={seconds}
```

## 📊 数据结构

### PlayerMetadata

```go
type PlayerMetadata struct {
    Player    string // 播放器名称
    TrackID   string // 歌曲ID
    Title     string // 歌曲标题
    Artist    string // 艺术家
    Album     string // 专辑
    Duration  int64  // 时长 (微秒)
    ArtUrl    string // 封面 URL
}
```

### LyricResult

```go
type LyricResult struct {
    Original    string // 原歌词
    Translation string // 翻译歌词
}
```

## 🎯 支持的播放器

| 播放器    | 支持度  | 备注            |
| --------- | ------- | --------------- |
| SPlayer   | ✅ 完全 | 自动提取歌曲 ID |
| MPD       | ✅ 支持 | 通过 MPRIS 接口 |
| VLC       | ✅ 支持 | 通过 MPRIS 接口 |
| Spotify   | ✅ 支持 | 通过 MPRIS 接口 |
| Audacious | ✅ 支持 | 通过 MPRIS 接口 |
| Rhythmbox | ✅ 支持 | 通过 MPRIS 接口 |

## 🚨 故障排除

### 错误：未找到 MPRIS 播放器

**原因**：没有运行的 MPRIS 兼容播放器

**解决**：

```bash
# 确保播放器正在运行
# 检查 D-Bus 名称
dbus-send --session --print-reply /org/freedesktop/DBus /org/freedesktop/DBus ListNames
```

### 错误：无法从播放器获取标题或艺术家

**原因**：元数据解析失败，可能是数据类型不匹配

**解决**：检查播放器是否正在播放歌曲

### 错误：网易云/Lrclib 未找到歌曲

**原因**：歌曲数据库中没有该歌曲或歌曲名称、艺术家不匹配

**解决**：

- 检查歌曲名称是否准确
- 尝试使用其他搜索词
- 如果是网易云失败，程序会自动尝试 lrclib

## 💡 使用建议

### 集成到 Waybar

在 `~/.config/waybar/config` 中添加：

```json
"custom/lyrics": {
    "format": "♪ {}",
    "exec": "/path/to/lyrics | head -1",
    "interval": 1
}
```

### 集成到其他工具

由于程序输出标准格式的歌词，可以轻松集成到：

- Waybar 小部件
- Polybar 模块
- 命令行脚本
- 其他系统工具

## 📝 环境变量

目前无特殊环境变量支持

## 🔐 隐私和安全

- ✅ 所有通信使用 HTTPS（API 端点）
- ✅ 仅向官方 API 服务器发送请求
- ✅ 不存储任何个人数据
- ✅ 完全开源，可审计

## 📄 许可证

[查看 LICENSE 文件](LICENSE)

## 🤝 贡献

欢迎提交问题报告和拉取请求！

## ⚠️ 注意

- 网易云 API 和 Lrclib API 政策可能会变化
- 请尊重版权所有者的权利
- 此项目仅供学习和个人使用

## 🔗 相关资源

- [MPRIS 规范](https://specifications.freedesktop.org/mpris-spec/2.2/)
- [D-Bus 文档](https://dbus.freedesktop.org/)
- [网易云音乐](https://music.163.com/)
- [Lrclib](https://lrclib.net/)

## 📧 联系方式

如有问题或建议，欢迎在 GitHub Issues 中讨论。
