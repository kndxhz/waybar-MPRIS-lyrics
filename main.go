package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/godbus/dbus/v5"
)

var base_163 = "https://music.163.com"
var base_lrclib = "https://lrclib.net"

// LyricData 定义歌词API响应结构
type LyricData struct {
	Lrc struct {
		Version int    `json:"version"`
		Lyric   string `json:"lyric"`
	} `json:"lrc"`
	Tlyric struct {
		Version int    `json:"version"`
		Lyric   string `json:"lyric"`
	} `json:"tlyric"`
	Code int `json:"code"`
}

// LyricResult 定义歌词返回结果
type LyricResult struct {
	Original   string // 原歌词
	Translation string // 翻译歌词
}

// SearchResult 定义搜索API响应结构
type SearchResult struct {
	Result struct {
		Songs []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"songs"`
		SongCount int `json:"songCount"`
	} `json:"result"`
	Code int `json:"code"`
}

// LrcLibResult 定义 lrclib 搜索响应
type LrcLibResult struct {
	ID          int    `json:"id"`
	TrackName   string `json:"trackName"`
	ArtistName  string `json:"artistName"`
	AlbumName   string `json:"albumName"`
	Duration    int    `json:"duration"`
	Instrumental bool  `json:"instrumental"`
	PlainLyrics string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

// PlayerMetadata 定义播放器元数据
type PlayerMetadata struct {
	Player    string // 播放器名称
	TrackID   string // 歌曲ID (splayer用)
	Title     string
	Artist    string
	Album     string
	Duration  int64  // 时长 (毫秒)
	ArtUrl    string
}

func main() {
	// 从播放器获取元数据
	metadata := getPlayerMetadata()
	if metadata == nil {
		fmt.Println("错误: 无法获取播放器信息")
		return
	}

	fmt.Printf("播放器: %s\n", metadata.Player)
	fmt.Printf("标题: %s\n", metadata.Title)
	fmt.Printf("艺术家: %s\n", metadata.Artist)
	if metadata.Album != "" {
		fmt.Printf("专辑: %s\n", metadata.Album)
	}

	var song_id string

	// 如果是 splayer，直接从 TrackID 提取歌曲ID
	if metadata.Player == "splayer" && metadata.TrackID != "" {
		// 从 '/com/splayer/track/3349609304' 提取数字ID
		re := regexp.MustCompile(`/(\d+)$`)
		matches := re.FindStringSubmatch(metadata.TrackID)
		if len(matches) > 1 {
			song_id = matches[1]
			fmt.Printf("从 splayer 提取的歌曲ID: %s\n", song_id)
		}
	}

	// 如果没有 song_id，尝试搜索
	if song_id == "" {
		song_id = searchSong(metadata.Title, metadata.Artist)
		if song_id == "" {
			fmt.Println("警告: 网易云未找到歌曲，尝试 lrclib...")
			// 尝试 lrclib 搜索
			durationSec := int(metadata.Duration / 1000) // 毫秒转秒
			lrcLibResult := searchLrcLib(metadata.Title, metadata.Artist, metadata.Album, durationSec)
			if lrcLibResult != nil {
				fmt.Println("\n=== 来自 lrclib 的歌词 ===")
				fmt.Println(lrcLibResult.Original)
				return
			}
			fmt.Println("错误: 两个数据源都未找到歌词")
			return
		}
		fmt.Printf("搜索得到的歌曲ID: %s\n", song_id)
	}

	// 获取歌词
	result := get_lyrics(song_id)
	if result != nil {
		fmt.Println("\n=== 原歌词 ===")
		fmt.Println(result.Original)
		if result.Translation != "" {
			fmt.Println("\n=== 翻译歌词 ===")
			fmt.Println(result.Translation)
		}
	} else {
		fmt.Println("错误: 无法获取歌词")
	}
}

// getPlayerMetadata 通过 D-Bus MPRIS 接口获取播放器元数据
func getPlayerMetadata() *PlayerMetadata {
	// 连接到 D-Bus
	conn, err := dbus.SessionBus()
	if err != nil {
		fmt.Println("错误: 连接 D-Bus 失败", err)
		return nil
	}
	defer conn.Close()

	// 查找所有 MPRIS 播放器服务
	var names []string
	err = conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		fmt.Println("错误: 列出 DBus 名称失败", err)
		return nil
	}

	var mediaPlayer dbus.BusObject
	var playerName string
	var foundPlayer bool

	// 首先查找 splayer，然后再找其他播放器
	var targetPlayer string
	for _, name := range names {
		if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			if strings.Contains(name, "splayer") {
				targetPlayer = name
				break
			}
		}
	}

	// 如果没找到 splayer，就用第一个 MPRIS 播放器
	if targetPlayer == "" {
		for _, name := range names {
			if strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
				targetPlayer = name
				break
			}
		}
	}

	if targetPlayer != "" {
		mediaPlayer = conn.Object(targetPlayer, "/org/mpris/MediaPlayer2")
		playerName = strings.TrimPrefix(targetPlayer, "org.mpris.MediaPlayer2.")
		foundPlayer = true
	}

	if !foundPlayer {
		fmt.Println("错误: 未找到 MPRIS 播放器")
		return nil
	}

	metadata := &PlayerMetadata{
		Player: playerName,
		Title:  "",
		Artist: "",
		Album:  "",
	}

	// 获取元数据 - 使用 GetAll 获取所有属性
	var allProps map[string]dbus.Variant
	err = mediaPlayer.Call("org.freedesktop.DBus.Properties.GetAll", 0, "org.mpris.MediaPlayer2.Player").Store(&allProps)
	if err != nil {
		fmt.Println("错误: 获取播放器属性失败", err)
		return nil
	}

	// 从 Metadata 属性中获取元数据
	var metadata_map map[string]dbus.Variant
	if metadataVar, ok := allProps["Metadata"]; ok {
		if m, ok := metadataVar.Value().(map[string]dbus.Variant); ok {
			metadata_map = m
		}
	}

	if metadata_map == nil {
		fmt.Println("错误: Metadata 为空或格式错误")
		return nil
	}

	// 解析元数据
	if val, ok := metadata_map["mpris:trackid"]; ok {
		if trackid, ok := val.Value().(dbus.ObjectPath); ok {
			metadata.TrackID = string(trackid)
		}
	}

	// 处理 xesam:title - 可能是 string 或 []string
	if val, ok := metadata_map["xesam:title"]; ok {
		v := val.Value()
		switch v := v.(type) {
		case string:
			metadata.Title = v
		case []string:
			if len(v) > 0 {
				metadata.Title = v[0]
			}
		case []interface{}:
			if len(v) > 0 {
				if str, ok := v[0].(string); ok {
					metadata.Title = str
				}
			}
		}
	}

	// 处理 xesam:artist - 可能是 string 或 []string
	if val, ok := metadata_map["xesam:artist"]; ok {
		v := val.Value()
		switch v := v.(type) {
		case string:
			metadata.Artist = v
		case []string:
			if len(v) > 0 {
				metadata.Artist = v[0]
			}
		case []interface{}:
			if len(v) > 0 {
				if str, ok := v[0].(string); ok {
					metadata.Artist = str
				}
			}
		}
	}

	if val, ok := metadata_map["xesam:album"]; ok {
		if album, ok := val.Value().(string); ok {
			metadata.Album = album
		}
	}

	if val, ok := metadata_map["mpris:length"]; ok {
		if length, ok := val.Value().(int64); ok {
			metadata.Duration = length
		}
	}

	if val, ok := metadata_map["mpris:artUrl"]; ok {
		if artUrl, ok := val.Value().(string); ok {
			metadata.ArtUrl = artUrl
		}
	}

	if metadata.Title == "" || metadata.Artist == "" {
		fmt.Println("错误: 无法从播放器获取标题或艺术家")
		return nil
	}

	return metadata
}

// searchSong 搜索歌曲，返回歌曲ID
func searchSong(title, artist string) string {
	// 构建搜索词：艺术家 标题
	query := artist + " " + title

	// 调用网易云搜索API
	encodedQuery := url.QueryEscape(query)
	searchURL := base_163 + "/api/search/get/web?s=" + encodedQuery + "&type=1&limit=1"

	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		fmt.Println("错误: 创建搜索请求失败", err)
		return ""
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("错误: 搜索请求失败", err)
		return ""
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("错误: 读取搜索响应失败", err)
		return ""
	}

	var searchResult SearchResult
	err = json.Unmarshal(body, &searchResult)
	if err != nil {
		fmt.Println("错误: 解析搜索响应失败", err)
		return ""
	}

	if searchResult.Code != 200 {
		fmt.Printf("错误: 搜索API返回错误代码 %d\n", searchResult.Code)
		return ""
	}

	if len(searchResult.Result.Songs) == 0 {
		fmt.Println("提示: 搜索未找到歌曲")
		return ""
	}

	// 返回第一个结果的ID
	song_id := fmt.Sprintf("%d", searchResult.Result.Songs[0].ID)
	return song_id
}

// searchLrcLib 在 lrclib 中搜索歌词
func searchLrcLib(trackName, artistName, albumName string, duration int) *LyricResult {
	// 构建查询参数
	params := url.Values{}
	params.Add("track_name", trackName)
	params.Add("artist_name", artistName)
	params.Add("album_name", albumName)
	params.Add("duration", fmt.Sprintf("%d", duration))

	searchURL := base_lrclib + "/api/get?" + params.Encode()

	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		fmt.Println("错误: 创建 lrclib 请求失败", err)
		return nil
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("错误: lrclib 请求失败", err)
		return nil
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("错误: 读取 lrclib 响应失败", err)
		return nil
	}

	// 处理 404
	if res.StatusCode == 404 {
		fmt.Println("提示: lrclib 未找到歌曲")
		return nil
	}

	if res.StatusCode != 200 {
		fmt.Printf("错误: lrclib 返回状态码 %d\n", res.StatusCode)
		return nil
	}

	var lrcLibResult LrcLibResult
	err = json.Unmarshal(body, &lrcLibResult)
	if err != nil {
		fmt.Println("错误: 解析 lrclib 响应失败", err)
		return nil
	}

	// 返回 lrclib 的歌词
	return &LyricResult{
		Original:   lrcLibResult.PlainLyrics,
		Translation: "", // lrclib 不提供翻译
	}
}

func get_lyrics(song_id string) *LyricResult {
	//获取网易云歌词接口
	url := base_163 + "/api/song/lyric?id=" + song_id + "&lv=1&tv=1"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		fmt.Println("错误: 创建请求失败", err)
		return nil
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("错误: 请求失败", err)
		return nil
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("错误: 读取响应失败", err)
		return nil
	}

	// 解析JSON
	var data LyricData
	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("错误: JSON解析失败", err)
		return nil
	}

	if data.Code != 200 {
		fmt.Println("错误: API返回错误代码", data.Code)
		return nil
	}

	// 返回解析结果
	return &LyricResult{
		Original:   data.Lrc.Lyric,
		Translation: data.Tlyric.Lyric,
	}
}