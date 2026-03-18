package main

import (
	"fmt"
	"io"
	"net/http"
)

var base_163 = "https://music.163.com"

func get_lyrics(song_id string) {

   url := base_163 + "/api/song/lyric?id=" + song_id + "&os=pc&lv=-1"
   method := "GET"

   client := &http.Client {
   }
   req, err := http.NewRequest(method, url, nil)

   if err != nil {
      fmt.Println(err)
      return
   }
   req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (HTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0")

   res, err := client.Do(req)
   if err != nil {
      fmt.Println(err)
      return
   }
   defer res.Body.Close()

   body, err := io.ReadAll(res.Body)
   if err != nil {
      fmt.Println(err)
      return
   }
   fmt.Println(string(body))
}