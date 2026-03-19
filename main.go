package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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

type WaybarOutput struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class,omitempty"`
}

type TimedLyricLine struct {
	TimeMs int64
	Text   string
}

type ParsedLyrics struct {
	Original    []TimedLyricLine
	Translation []TimedLyricLine
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
	Duration  int64  // 时长 (微秒)
	Position  int64  // 播放位置 (微秒)
	ArtUrl    string
}

func main() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	lastSongKey := ""
	lastRendered := ""
	var currentLyrics *ParsedLyrics

	for {
		metadata := getPlayerMetadata()
		if metadata == nil {
			empty := renderWaybar(WaybarOutput{Text: "", Tooltip: "", Class: "stopped"})
			if empty != lastRendered {
				fmt.Println(empty)
				lastRendered = empty
			}
			lastSongKey = ""
			<-ticker.C
			continue
		}

		songKey := buildSongKey(metadata)
		if songKey != lastSongKey {
			currentLyrics = resolveLyrics(metadata)
			lastSongKey = songKey
		}

		text := buildCurrentLyricText(metadata, currentLyrics)
		if text == "" {
			text = buildText(metadata)
		}

		out := WaybarOutput{
			Text:    text,
			Tooltip: buildText(metadata),
			Class:   "lyrics",
		}
		rendered := renderWaybar(out)
		if rendered != lastRendered {
			fmt.Println(rendered)
			lastRendered = rendered
		}

		<-ticker.C
	}
}

func buildSongKey(metadata *PlayerMetadata) string {
	return strings.Join([]string{
		metadata.Player,
		metadata.TrackID,
		metadata.Title,
		metadata.Artist,
		metadata.Album,
	}, "|")
}

func buildText(metadata *PlayerMetadata) string {
	if metadata.Artist == "" {
		return metadata.Title
	}
	return metadata.Artist + " - " + metadata.Title
}

func renderWaybar(out WaybarOutput) string {
	buf, err := json.Marshal(out)
	if err != nil {
		return `{"text":"","tooltip":"","class":"error"}`
	}
	return string(buf)
}

func resolveLyrics(metadata *PlayerMetadata) *ParsedLyrics {
	var songID string

	if strings.HasPrefix(metadata.Player, "splayer") && metadata.TrackID != "" {
		re := regexp.MustCompile(`/(\d+)$`)
		matches := re.FindStringSubmatch(metadata.TrackID)
		if len(matches) > 1 {
			songID = matches[1]
		}
	}

	if songID == "" {
		songID = searchSong(metadata.Title, metadata.Artist)
	}

	if songID != "" {
		result := get_lyrics(songID)
		if result != nil {
			if strings.TrimSpace(result.Translation) == "" {
				orig, trans := parseLRCWithInlineTranslation(result.Original)
				if len(trans) > 0 {
					return &ParsedLyrics{
						Original:    orig,
						Translation: trans,
					}
				}
			}

			return &ParsedLyrics{
				Original:    parseLRC(result.Original),
				Translation: parseLRC(result.Translation),
			}
		}
	}

	durationSec := int(metadata.Duration / 1000000)
	lrcLibResult := searchLrcLib(metadata.Title, metadata.Artist, metadata.Album, durationSec)
	if lrcLibResult != nil {
		if lrcLibResult.SyncedLyrics != "" {
			return &ParsedLyrics{Original: parseLRC(lrcLibResult.SyncedLyrics)}
		}
		if lrcLibResult.PlainLyrics != "" {
			return &ParsedLyrics{Original: []TimedLyricLine{{TimeMs: 0, Text: lrcLibResult.PlainLyrics}}}
		}
	}

	return nil
}

func parseLRC(content string) []TimedLyricLine {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	lineRe := regexp.MustCompile(`\[(\d{2}):(\d{2})(?:\.(\d{1,3}))?\]`)
	var result []TimedLyricLine

	for _, line := range strings.Split(content, "\n") {
		matches := lineRe.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		text := strings.TrimSpace(lineRe.ReplaceAllString(line, ""))
		for _, m := range matches {
			mm, err1 := strconv.Atoi(m[1])
			ss, err2 := strconv.Atoi(m[2])
			if err1 != nil || err2 != nil {
				continue
			}

			ms := 0
			if m[3] != "" {
				frac := m[3]
				switch len(frac) {
				case 1:
					frac += "00"
				case 2:
					frac += "0"
				}
				ms, _ = strconv.Atoi(frac)
			}

			timeMs := int64(mm*60*1000 + ss*1000 + ms)
			result = append(result, TimedLyricLine{TimeMs: timeMs, Text: text})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimeMs < result[j].TimeMs
	})

	return result
}

func parseLRCWithInlineTranslation(content string) ([]TimedLyricLine, []TimedLyricLine) {
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	lineRe := regexp.MustCompile(`\[(\d{2}):(\d{2})(?:\.(\d{1,3}))?\]`)
	var original []TimedLyricLine
	var translation []TimedLyricLine

	for _, line := range strings.Split(content, "\n") {
		matches := lineRe.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		text := strings.TrimSpace(lineRe.ReplaceAllString(line, ""))
		parts := strings.SplitN(text, "|", 2)
		origText := strings.TrimSpace(parts[0])
		transText := ""
		if len(parts) == 2 {
			transText = strings.TrimSpace(parts[1])
		}

		for _, m := range matches {
			mm, err1 := strconv.Atoi(m[1])
			ss, err2 := strconv.Atoi(m[2])
			if err1 != nil || err2 != nil {
				continue
			}

			ms := 0
			if m[3] != "" {
				frac := m[3]
				switch len(frac) {
				case 1:
					frac += "00"
				case 2:
					frac += "0"
				}
				ms, _ = strconv.Atoi(frac)
			}

			timeMs := int64(mm*60*1000 + ss*1000 + ms)
			if origText != "" {
				original = append(original, TimedLyricLine{TimeMs: timeMs, Text: origText})
			}
			if transText != "" {
				translation = append(translation, TimedLyricLine{TimeMs: timeMs, Text: transText})
			}
		}
	}

	sort.Slice(original, func(i, j int) bool {
		return original[i].TimeMs < original[j].TimeMs
	})
	sort.Slice(translation, func(i, j int) bool {
		return translation[i].TimeMs < translation[j].TimeMs
	})

	return original, translation
}

func currentLineAt(lines []TimedLyricLine, positionMs int64) string {
	if len(lines) == 0 {
		return ""
	}

	idx := sort.Search(len(lines), func(i int) bool {
		return lines[i].TimeMs > positionMs
	})
	if idx == 0 {
		return ""
	}

	return strings.TrimSpace(lines[idx-1].Text)
}

func buildCurrentLyricText(metadata *PlayerMetadata, lyrics *ParsedLyrics) string {
	if lyrics == nil {
		return ""
	}

	positionMs := metadata.Position / 1000
	original := currentLineAt(lyrics.Original, positionMs)
	translation := currentLineAt(lyrics.Translation, positionMs)
 
	if original == "" && translation == "" {
		return ""
	}

	if translation == "" || translation == original {
		return original
	}

	if original == "" {
		return translation
	}

	// Use Pango markup for multiline: <span size='small'>...</span>
	// But simply \n works if Waybar label supports it.
	// Let's try to return Text as is, but maybe the issue is newlines being stripped by Waybar or not rendered if height is small.
	// A common trick is to use <span rise='-1000'> or separate lines.
	// But let's first ensure we are flushing stdout.
	return original + "\n" + translation
}

// getPlayerMetadata 通过 D-Bus MPRIS 接口获取播放器元数据
func getPlayerMetadata() *PlayerMetadata {
	// 连接到 D-Bus
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil
	}
	defer conn.Close()

	// 查找所有 MPRIS 播放器服务
	var names []string
	err = conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
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
		return nil
	}

	if val, ok := allProps["Position"]; ok {
		if pos, ok := val.Value().(int64); ok {
			metadata.Position = pos
		}
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
		return ""
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return ""
	}

	var searchResult SearchResult
	err = json.Unmarshal(body, &searchResult)
	if err != nil {
		return ""
	}

	if searchResult.Code != 200 {
		return ""
	}

	if len(searchResult.Result.Songs) == 0 {
		return ""
	}

	// 返回第一个结果的ID
	song_id := strconv.Itoa(searchResult.Result.Songs[0].ID)
	return song_id
}

// searchLrcLib 在 lrclib 中搜索歌词
func searchLrcLib(trackName, artistName, albumName string, duration int) *LrcLibResult {
	// 构建查询参数
	params := url.Values{}
	params.Add("track_name", trackName)
	params.Add("artist_name", artistName)
	params.Add("album_name", albumName)
	params.Add("duration", strconv.Itoa(duration))

	searchURL := base_lrclib + "/api/get?" + params.Encode()

	client := &http.Client{}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil
	}

	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil
	}

	// 处理 404
	if res.StatusCode == 404 {
		return nil
	}

	if res.StatusCode != 200 {
		return nil
	}

	var lrcLibResult LrcLibResult
	err = json.Unmarshal(body, &lrcLibResult)
	if err != nil {
		return nil
	}

	return &lrcLibResult
}

func get_lyrics(song_id string) *LyricResult {
	//获取网易云歌词接口
	url := base_163 + "/api/song/lyric?id=" + song_id + "&lv=1&tv=1"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)

	if err != nil {
		return nil
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil
	}

	// 解析JSON
	var data LyricData
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil
	}

	if data.Code != 200 {
		return nil
	}

	// 返回解析结果
	return &LyricResult{
		Original:   data.Lrc.Lyric,
		Translation: data.Tlyric.Lyric,
	}
}